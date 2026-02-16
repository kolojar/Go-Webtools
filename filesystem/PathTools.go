package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

/*
JoinPathSecure joins path and checks if it hasnt escaped from basePath. Returns error os.ErrInvalid if path escaped. PAth must exist (only last part does not have to)
*/
func JoinPathSecure(basePath string, addedPath string) (string, error) {
	//Get absolute path of base
	basePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}

	//Join
	joinedPath := filepath.Join(basePath, addedPath)
	joinedPath = filepath.Clean(joinedPath)

	//Resolve symlinks
	basePathReal, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		return "", err
	}
	joinedPathReal, err := filepath.EvalSymlinks(joinedPath)
	if err != nil {
		//File or folder does not exist -> Check parentPath
		parentPath := filepath.Dir(joinedPath)
		parentPathReal, err := filepath.EvalSymlinks(parentPath)
		if err != nil {
			return "", err
		}
		joinedPathReal = filepath.Join(parentPathReal, filepath.Base(joinedPath))
	}

	//Check if path escaped
	if !strings.HasPrefix(joinedPathReal, basePathReal+string(os.PathSeparator)) && joinedPathReal != basePathReal {
		return "", os.ErrInvalid
	}
	return joinedPathReal, nil
}
