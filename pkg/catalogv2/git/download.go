package git

import (
	plumbing "github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

const (
	stateDir  = "management-state/git-repo"
	staticDir = "/var/lib/rancher-data/local-catalogs/v2"
)

// Ensure builds the configuration for a should-existing repo and makes sure it is cloned or reseted to the latest commit
func Ensure(secret *corev1.Secret, namespace, name, gitURL, commit string, insecureSkipTLS bool, caBundle []byte) error {
	if commit == "" {
		return nil
	}

	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return err
	}

	return git.EnsureClonedRepo(commit)
}

// EnsureClonedRepo will check if repo is cloned, if not will clone and reset to the latest commit.
// If reseting to the latest commit is not possible it will fetch and try to reset
func (er *extendedRepo) EnsureClonedRepo(commit string) error {

	err := er.cloneOrOpen("")
	if err != nil {
		return err
	}

	commitHASH := plumbing.NewHash(commit)

	// Try to reset to the given commit, if success exit
	err = er.hardReset(commitHASH)
	if err == nil {
		return nil
	}
	// If we do not have the commit locally, fetch and reset
	return er.fetchAndReset(commitHASH, "")
}

// Head builds the configuration for a new repo which will be cloned for the first time
func Head(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	return git.CloneHead(branch)
}

// CloneHead clones the HEAD of a git branch and return the commit hash of the HEAD.
func (er *extendedRepo) CloneHead(branch string) (string, error) {
	err := er.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	zeroHash := plumbing.NewHash("")
	err = er.hardReset(zeroHash)
	if err != nil {
		return "", err
	}

	commit, err := er.getCurrentCommit()
	return commit.String(), err
}

// Update builds the configuration to update an existing repository
func Update(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	if isBundled(git.GetConfig()) && settings.SystemCatalog.Get() == "bundled" {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	commit, err := git.UpdateToLatestRef(branch)

	if err != nil && isBundled(git.GetConfig()) {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	return commit, err
}

// UpdateToLatestRef will check if repository exists, if exists will check for latest commit and update to it.
// If the repository does not exist will try cloning again.
func (er *extendedRepo) UpdateToLatestRef(branch string) (string, error) {
	err := er.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	zeroHash := plumbing.NewHash("")
	err = er.hardReset(zeroHash)
	if err != nil {
		return "", err
	}

	commit, err := er.getCurrentCommit()
	if err != nil {
		return commit.String(), err
	}

	lastCommit, err := er.getLastCommitHash(branch, commit)
	if err != nil || lastCommit == commit {
		return commit.String(), err
	}

	err = er.fetchAndReset(lastCommit, branch)
	if err != nil {
		return commit.String(), err
	}

	lastCommitRef, err := er.getCurrentCommit()
	lastCommitHashStr := lastCommitRef.String()

	return lastCommitHashStr, err
}
