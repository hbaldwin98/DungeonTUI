package gitsync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const origin = "origin"

// Client mirrors a Snapshot into a git repository and back. SQLite remains
// the operational store; this client is backup and transport between machines.
type Client struct {
	Dir    string
	Remote string
	Branch string
	Git    Git
}

// Result is a short description of what a push or pull did.
type Result struct {
	Message string
	Commit  string
}

// ConflictError means git could not merge two edits of the same entity file.
// The local sqlite workspace is left untouched.
type ConflictError struct {
	Files []string
}

func (e *ConflictError) Error() string {
	if e == nil {
		return "git merge conflict"
	}
	return fmt.Sprintf("git merge conflict in %s; local workspace was not changed", strings.Join(e.Files, ", "))
}

func (c Client) branch() string {
	if strings.TrimSpace(c.Branch) != "" {
		return c.Branch
	}
	if isGitRepo(c.Dir) {
		if name, err := c.git("rev-parse", "--abbrev-ref", "HEAD"); err == nil && name != "" && name != "HEAD" {
			return name
		}
	}
	return "main"
}

func (c Client) git(args ...string) (string, error) {
	return c.Git.Run(c.Dir, args...)
}

// Init creates or clones the local mirror and points origin at Remote.
func (c Client) Init() error {
	if strings.TrimSpace(c.Remote) == "" {
		return fmt.Errorf("sync remote is required")
	}
	if strings.TrimSpace(c.Dir) == "" {
		return fmt.Errorf("sync directory is required")
	}
	if !c.Git.Available() {
		return fmt.Errorf("%w: %q not found", ErrUnavailable, c.Git.binary())
	}
	if isGitRepo(c.Dir) {
		return c.setRemote()
	}
	if err := os.MkdirAll(filepath.Dir(c.Dir), 0o755); err != nil {
		return fmt.Errorf("create sync directory: %w", err)
	}
	if _, err := os.Stat(c.Dir); err == nil {
		if empty, emptyErr := dirEmpty(c.Dir); emptyErr != nil {
			return emptyErr
		} else if !empty {
			return fmt.Errorf("sync directory %s exists and is not an empty git repository", c.Dir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat sync directory: %w", err)
	}
	if _, err := c.Git.Run("", "clone", c.Remote, c.Dir); err == nil {
		return c.setRemote()
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("create sync directory: %w", err)
	}
	if _, err := c.Git.Run(c.Dir, "init", "-b", c.branch()); err != nil {
		return err
	}
	return c.setRemote()
}

func (c Client) setRemote() error {
	if _, err := c.git("remote", "set-url", origin, c.Remote); err == nil {
		return nil
	}
	_, err := c.git("remote", "add", origin, c.Remote)
	return err
}

func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func dirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

// Push writes the snapshot, commits if it changed, and uploads to origin.
func (c Client) Push(snap Snapshot) (Result, error) {
	if err := c.ensure(); err != nil {
		return Result{}, err
	}
	if err := c.fetch(); err != nil {
		return Result{}, err
	}
	ahead, err := c.remoteAhead()
	if err != nil {
		return Result{}, err
	}
	if ahead {
		return Result{}, fmt.Errorf("remote has new commits; run dungeon sync pull first")
	}
	if err := Write(c.Dir, snap); err != nil {
		return Result{}, err
	}
	committed, err := c.commitIfNeeded()
	if err != nil {
		return Result{}, err
	}
	if err := c.push(); err != nil {
		return Result{}, err
	}
	hash, _ := c.git("rev-parse", "--short", "HEAD")
	if !committed {
		return Result{Message: "already up to date", Commit: hash}, nil
	}
	return Result{Message: "pushed workspace", Commit: hash}, nil
}

// Pull integrates origin into the local mirror and returns the snapshot the
// repository now holds. When both machines edited, distinct entity files merge
// and the same file conflicts. force discards local unpushed git commits and
// takes origin as-is.
func (c Client) Pull(local Snapshot, force bool) (Snapshot, Result, error) {
	if err := c.ensure(); err != nil {
		return Snapshot{}, Result{}, err
	}
	if err := c.fetch(); err != nil {
		return Snapshot{}, Result{}, err
	}
	if force {
		if err := c.takeRemote(); err != nil {
			return Snapshot{}, Result{}, err
		}
		snap, err := Read(c.Dir)
		if err != nil {
			return Snapshot{}, Result{}, missingSnapshot(err)
		}
		hash, _ := c.git("rev-parse", "--short", "HEAD")
		return snap, Result{Message: "reset workspace from remote", Commit: hash}, nil
	}
	ahead, err := c.remoteAhead()
	if err != nil {
		return Snapshot{}, Result{}, err
	}
	head, headErr := Read(c.Dir)
	localDirty := hasCampaignData(local) && (headErr != nil || !snapshotsEqual(local, head))
	if ahead && localDirty {
		if err := c.mergeLocal(local); err != nil {
			return Snapshot{}, Result{}, err
		}
		snap, err := Read(c.Dir)
		if err != nil {
			return Snapshot{}, Result{}, err
		}
		hash, _ := c.git("rev-parse", "--short", "HEAD")
		return snap, Result{Message: "merged remote workspace", Commit: hash}, nil
	}
	if ahead {
		if err := c.fastForward(); err != nil {
			return Snapshot{}, Result{}, err
		}
		snap, err := Read(c.Dir)
		if err != nil {
			return Snapshot{}, Result{}, missingSnapshot(err)
		}
		hash, _ := c.git("rev-parse", "--short", "HEAD")
		return snap, Result{Message: "pulled workspace", Commit: hash}, nil
	}
	if headErr != nil {
		return Snapshot{}, Result{}, missingSnapshot(headErr)
	}
	hash, _ := c.git("rev-parse", "--short", "HEAD")
	if localDirty {
		return local, Result{Message: "nothing to pull; run dungeon sync push to upload local changes", Commit: hash}, nil
	}
	return head, Result{Message: "already up to date", Commit: hash}, nil
}

func missingSnapshot(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("repository has no Dungeon workspace yet; push from a machine that has one")
	}
	return err
}

func (c Client) ensure() error {
	if !isGitRepo(c.Dir) {
		return c.Init()
	}
	if strings.TrimSpace(c.Remote) != "" {
		return c.setRemote()
	}
	return nil
}

func (c Client) fetch() error {
	_, err := c.git("fetch", origin)
	if err == nil {
		return nil
	}
	// An empty remote has no heads yet; the first push creates them.
	heads, lsErr := c.git("ls-remote", "--heads", origin)
	if lsErr == nil && strings.TrimSpace(heads) == "" {
		return nil
	}
	return err
}

func (c Client) remoteRef() string {
	return origin + "/" + c.branch()
}

func (c Client) hasHEAD() bool {
	_, err := c.git("rev-parse", "--verify", "HEAD")
	return err == nil
}

func (c Client) hasRemoteBranch() bool {
	_, err := c.git("rev-parse", "--verify", c.remoteRef())
	return err == nil
}

func (c Client) remoteAhead() (bool, error) {
	if !c.hasRemoteBranch() {
		return false, nil
	}
	if !c.hasHEAD() {
		return true, nil
	}
	local, err := c.git("rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	remote, err := c.git("rev-parse", c.remoteRef())
	if err != nil {
		return false, err
	}
	if local == remote {
		return false, nil
	}
	if _, err := c.git("merge-base", "--is-ancestor", "HEAD", c.remoteRef()); err == nil {
		return true, nil
	}
	if _, err := c.git("merge-base", "--is-ancestor", c.remoteRef(), "HEAD"); err == nil {
		return false, nil
	}
	return true, nil
}

func (c Client) commitIfNeeded() (bool, error) {
	if _, err := c.git("add", "-A", "--", workspaceDir); err != nil {
		return false, err
	}
	if _, err := c.git("diff", "--cached", "--quiet"); err == nil {
		return false, nil
	}
	if _, err := c.git("commit", "-m", "Sync Dungeon workspace"); err != nil {
		return false, err
	}
	return true, nil
}

func (c Client) push() error {
	_, err := c.git("push", "-u", origin, "HEAD:"+c.branch())
	return err
}

func (c Client) takeRemote() error {
	if !c.hasRemoteBranch() {
		return fmt.Errorf("repository has no Dungeon workspace yet; push from a machine that has one")
	}
	_, err := c.git("checkout", "-B", c.branch(), c.remoteRef())
	return err
}

func (c Client) fastForward() error {
	if !c.hasRemoteBranch() {
		return fmt.Errorf("repository has no Dungeon workspace yet; push from a machine that has one")
	}
	if !c.hasHEAD() {
		_, err := c.git("checkout", "-B", c.branch(), c.remoteRef())
		return err
	}
	_, err := c.git("merge", "--ff-only", c.remoteRef())
	return err
}

func (c Client) mergeLocal(local Snapshot) error {
	if err := Write(c.Dir, local); err != nil {
		return err
	}
	if _, err := c.commitIfNeeded(); err != nil {
		return err
	}
	if _, err := c.git("merge", "--no-edit", c.remoteRef()); err == nil {
		return nil
	}
	files, _ := c.git("diff", "--name-only", "--diff-filter=U")
	_, _ = c.git("merge", "--abort")
	conflicted := splitFiles(files)
	if len(conflicted) == 0 {
		conflicted = []string{workspaceDir}
	}
	return &ConflictError{Files: conflicted}
}

func splitFiles(out string) []string {
	if strings.TrimSpace(out) == "" {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}
