package filesystem

import (
	"io"

	"github.com/kolojar/Go-Webtools/database"
	"github.com/kolojar/Go-Webtools/helpertools"
)

// nodeType is helper type for checking type of interface contained node
type nodeType bool

const file nodeType = false
const folder nodeType = true

// VirtualFileSystem creates virtual file system based on OS filesystem package. It uses JournalingRAMDatabase in binary tree format for storing data.
type VirtualFileSystem struct {
	path                           string
	journalItemCountBeforeAutosave uint32
	root                           virtualFileSystemFolderNode
	folderNodes                    helpertools.SafeMap[string, virtualFileSystemFolderNode]
	Logger                         helpertools.ConsoleLogger
}

// virtualFileSystemNode is interface node of file system
type virtualFileSystemNode interface {
	getNodeType() nodeType
}

// virtualFileSystemFolderEntryNode is node for referencing other folder nodes
type virtualFileSystemFolderEntryNode struct {
	name string
}

// getNodeType gets node type of folder entry (folder)
func (entry virtualFileSystemFolderEntryNode) getNodeType() nodeType {
	return folder
}

// virtualFilesSystemNode is base node of file system
type virtualFileSystemNodeBase struct {
}

// virtualFileSystemFileNode is node of file system for file representation
type virtualFileSystemFileNode struct {
	base virtualFileSystemNodeBase
}

// getNodeType gets node type of file entry (file)
func (entry virtualFileSystemFileNode) getNodeType() nodeType {
	return file
}

// virtualFileSystemFolderNode is node of file system for folder representation
type virtualFileSystemFolderNode struct {
	base  virtualFileSystemNodeBase
	nodes database.JournalingDatabase[virtualFileSystemNode]
}

// NewVirtualFileSystem creates new VirtualFileSystem but does not load any database of it. Set journalItemCountBeforeAutosave to 0 for disabled autosave
func NewVirtualFileSystem(path string, journalItemCountBeforeAutosave uint32) (*VirtualFileSystem, error) {
	//Create VFS
	vfs := &VirtualFileSystem{path: path, Logger: helpertools.MakeConsoleLogger("VFS"), journalItemCountBeforeAutosave: journalItemCountBeforeAutosave}

	//Load root node
	nodes, err := database.NewJournalingRAMDatabase[virtualFileSystemNode](path, journalItemCountBeforeAutosave, database.ConvertAnyToBytesDB)
	vfs.root = virtualFileSystemFolderNode{}
}

func (vfs *VirtualFileSystem) parseNodeDBFunc(reader io.Reader) (virtualFileSystemNode, error) {
	return database.ParseAnyDB[virtualFileSystemNode](reader, true)
}

func (vfs *VirtualFileSystem) Load() error {

}
