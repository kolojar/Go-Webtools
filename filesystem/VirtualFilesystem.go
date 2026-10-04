package filesystem

import (
	"github.com/kolojar/Go-Webtools/database"
	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
 * nodeType is helper type for checking type of interface contained node
 */
type nodeType bool

const file nodeType = false
const folder nodeType = true

/*
 * VirtualFileSystem creates virtual file system based on existing one. It uses JournalingRAMDatabase in binary tree format for storing data.
 */
type VirtualFileSystem struct {
	root        virtualFileSystemFolderNode
	folderNodes helpertools.SafeMap[string, virtualFileSystemFolderNode]
}

/*
 * virtualFileSystemNode is interface node of file system
 */
type virtualFileSystemNode interface {
	getNodeType() nodeType
}

/*
 * virtualFileSystemFolderEntryNode is node for referencing other folder nodes
 */
type virtualFileSystemFolderEntryNode struct {
	name string
}

/*
 * getNodeType gets node type of folder entry (folder)
 */
func (entry virtualFileSystemFolderEntryNode) getNodeType() nodeType {
	return folder
}

/*
 * virtualFilesSystemNode is base node of file system
 */
type virtualFileSystemNodeBase struct {
}

/*
 * virtualFileSystemFileNode is node of file system for file representation
 */
type virtualFileSystemFileNode struct {
	base virtualFileSystemNodeBase
}

/*
 * getNodeType gets node type of file entry (file)
 */
func (entry virtualFileSystemFileNode) getNodeType() nodeType {
	return file
}

/*
 * virtualFileSystemFolderNode is node of file system for folder representation
 */
type virtualFileSystemFolderNode struct {
	base  virtualFileSystemNodeBase
	nodes database.JournalingRAMDatabase[virtualFileSystemNode]
}
