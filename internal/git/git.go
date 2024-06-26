package git

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gogit "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v48/github"

	"github.com/gravitational/gamma/internal/action"
)

type Git interface {
	GetChangedFiles() ([]string, error)
	TagExists(a action.Action) (bool, error)
	DeployAction(a action.Action, pushTags bool) error
}

type git struct {
	repo *gogit.Repository
	gh   *github.Client
}

func New(wd string) (Git, error) {
	repo, err := gogit.PlainOpen(wd)
	if err != nil {
		return nil, fmt.Errorf("the current directory is not a git repo: %v", err)
	}

	gh, err := createGithubClient()
	if err != nil {
		return nil, err
	}

	return &git{repo, gh}, nil
}

func createGithubClient() (*github.Client, error) {
	if os.Getenv("GITHUB_APP_PRIVATE_KEY") == "" {
		return nil, errors.New("set your Github app's private key as GITHUB_APP_PRIVATE_KEY")
	}

	privateKey := strings.ReplaceAll(os.Getenv("GITHUB_APP_PRIVATE_KEY"), "\\n", "\n")

	if os.Getenv("GITHUB_APP_ID") == "" {
		return nil, errors.New("set your Github app's ID as GITHUB_APP_ID")
	}

	appID, err := strconv.Atoi(os.Getenv("GITHUB_APP_ID"))
	if err != nil {
		return nil, errors.New("the Github app ID should be a number")
	}

	if os.Getenv("GITHUB_APP_INSTALLATION_ID") == "" {
		return nil, errors.New("set your Github app's installation ID as GITHUB_APPID")
	}

	appInstallationID, err := strconv.Atoi(os.Getenv("GITHUB_APP_INSTALLATION_ID"))
	if err != nil {
		return nil, errors.New("the Github app installation ID should be a number")
	}

	itr, err := ghinstallation.New(http.DefaultTransport, int64(appID), int64(appInstallationID), []byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("could not authenticate with Github: %v", err)
	}

	return github.NewClient(&http.Client{Transport: itr}), nil
}

func (g *git) TagExists(a action.Action) (bool, error) {
	tags, _, err := g.gh.Repositories.ListTags(context.Background(), a.Owner(), a.RepoName(), nil)
	if err != nil {
		return false, fmt.Errorf("could not fetch tags: %v", err)
	}

	// iterate over all tags, return true if the tag exists
	for _, t := range tags {
		if *t.Name == fmt.Sprintf("v%s", a.Version()) {
			return true, nil
		}
	}

	return false, nil
}

func (g *git) GetChangedFiles() ([]string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get HEAD: %v", err)
	}

	commit, err := g.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not get the HEAD commit: %v", err)
	}

	parentHash := commit.ParentHashes[0]
	parent, err := g.repo.CommitObject(parentHash)
	if err != nil {
		return nil, fmt.Errorf("could not get the parent commit: %v", err)
	}

	patch, err := parent.Patch(commit)
	if err != nil {
		return nil, fmt.Errorf("could not get the parent patch: %v", err)
	}

	changedFiles := make(map[string]struct{})

	for _, p := range patch.FilePatches() {
		from, to := p.Files()

		if from != nil {
			changedFiles[from.Path()] = struct{}{}
		}
		if to != nil {
			changedFiles[to.Path()] = struct{}{}
		}
	}

	var files []string
	for file := range changedFiles {
		files = append(files, file)
	}

	return files, nil
}

func (g *git) DeployAction(a action.Action, pushTags bool) error {
	ref, err := g.getRef(context.Background(), a)
	if err != nil {
		return fmt.Errorf("could not create git ref: %v", err)
	}

	if pushTags {
		// make sure tag doesn't already exist
		tagExists, err := g.TagExists(a)
		if err != nil {
			return fmt.Errorf("could not verify if tag exists: %v", err)
		}

		if tagExists {
			return fmt.Errorf("tag already exists: v%v", a.Version())
		}
	}

	tree, err := g.getTree(context.Background(), ref, a)
	if err != nil {
		return fmt.Errorf("could not create git tree: %v", err)
	}

	newCommit, err := g.pushCommit(context.Background(), ref, tree, a)
	if err != nil {
		return fmt.Errorf("could not push changes: %v", err)
	}

	if pushTags {
		if err := g.pushTag(context.Background(), a, newCommit); err != nil {
			return fmt.Errorf("could not push tag: %v", err)
		}
	}

	return nil
}

func (g *git) getTree(ctx context.Context, ref *github.Reference, a action.Action) (*github.Tree, error) {
	var entries []*github.TreeEntry

	ferr := filepath.Walk(a.OutputDirectory(),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("could not read %s: %v", path, err)
			}

			p, err := filepath.Rel(a.OutputDirectory(), path)
			if err != nil {
				return fmt.Errorf("could not resolve relative path between %s and %s: %v", a.OutputDirectory(), path, err)
			}

			entry := &github.TreeEntry{
				Path:    github.String(p),
				Type:    github.String("blob"),
				Content: github.String(string(content)),
				Mode:    github.String("100644"),
			}

			entries = append(entries, entry)

			return nil
		})

	if ferr != nil {
		return nil, ferr
	}

	tree, _, err := g.gh.Git.CreateTree(ctx, a.Owner(), a.RepoName(), *ref.Object.SHA, entries)

	return tree, err
}

func (g *git) getRef(ctx context.Context, a action.Action) (*github.Reference, error) {
	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get HEAD: %v", err)
	}

	ref, _, err := g.gh.Git.GetRef(ctx, a.Owner(), a.RepoName(), head.Name().String())
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func (g *git) pushCommit(ctx context.Context, ref *github.Reference, tree *github.Tree, a action.Action) (*github.Commit, error) {
	parent, _, err := g.gh.Repositories.GetCommit(ctx, a.Owner(), a.RepoName(), *ref.Object.SHA, nil)
	if err != nil {
		return nil, err
	}

	parent.Commit.SHA = parent.SHA

	head, err := g.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get HEAD: %v", err)
	}

	c, err := g.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not get the HEAD commit: %v", err)
	}

	commit := &github.Commit{
		Message: github.String(c.Message),
		Tree:    tree,
		Parents: []*github.Commit{parent.Commit},
	}

	newCommit, _, err := g.gh.Git.CreateCommit(ctx, a.Owner(), a.RepoName(), commit)
	if err != nil {
		return nil, err
	}

	ref.Object.SHA = newCommit.SHA
	_, _, err = g.gh.Git.UpdateRef(ctx, a.Owner(), a.RepoName(), ref, false)
	if err != nil {
		return nil, err
	}

	return newCommit, nil
}

func (g *git) pushTag(ctx context.Context, a action.Action, newCommit *github.Commit) error {
	tagString := fmt.Sprintf("v%v", a.Version())
	tag := &github.Tag{
		Tag:     github.String(tagString),
		Message: github.String(fmt.Sprintf("Tag for version %s", a.Version())),
		Object:  &github.GitObject{SHA: github.String(*newCommit.SHA), Type: github.String("commit")},
	}

	_, _, err := g.gh.Git.CreateTag(ctx, a.Owner(), a.RepoName(), tag)
	if err != nil {
		return fmt.Errorf("could not create the tag: %v", err)
	}

	refTag := &github.Reference{Ref: github.String("refs/tags/" + tagString), Object: &github.GitObject{SHA: github.String(*newCommit.SHA)}}
	_, _, err = g.gh.Git.CreateRef(ctx, a.Owner(), a.RepoName(), refTag)
	if err != nil {
		return fmt.Errorf("could not create the reference for tag: %v", err)
	}
	return nil
}
