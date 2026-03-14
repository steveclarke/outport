package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// Info holds information about the current git worktree.
type Info struct {
	Instance   string
	IsWorktree bool
}

// Detect determines whether dir is a git worktree or main checkout.
// In a worktree, .git is a file containing "gitdir: /path/to/main/.git/worktrees/<name>".
// In a main checkout, .git is a directory. Returns the instance name: the worktree
// directory name, or "main" for the primary checkout.
func Detect(dir string) (*Info, error) {
	gitPath := filepath.Join(dir, ".git")
	fi, err := os.Lstat(gitPath)
	if err != nil {
		return &Info{Instance: "main", IsWorktree: false}, nil
	}

	if fi.IsDir() {
		return &Info{Instance: "main", IsWorktree: false}, nil
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return nil, err
	}

	line := strings.TrimSpace(string(data))
	gitdir := strings.TrimPrefix(line, "gitdir: ")
	name := filepath.Base(gitdir)

	return &Info{Instance: name, IsWorktree: true}, nil
}
