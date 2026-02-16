package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var iNVALID_NAMES = [...]string{"..", "."}

/*
Checks if path constains some invalid names (server protection) -> Returns TRUE if value is INVALID
! Must be present in every operation with files on server !
*/
func CheckInvalidNames(path string) error {
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Path does not start with /")
		return os.ErrInvalid
	}
	split := strings.Split(path, "/")
	for i := 0; i < len(iNVALID_NAMES); i++ {
		for k := 0; k < len(split); k++ {
			if strings.EqualFold(split[k], iNVALID_NAMES[i]) {
				fmt.Println("Found invalid name: " + iNVALID_NAMES[i] + " in: " + path)
				return os.ErrInvalid
			}
		}
	}
	return nil
}

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
