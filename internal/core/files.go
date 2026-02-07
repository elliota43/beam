package core

type Node interface {
	Path() string
	Name() string
}

type File struct {
	path string
	name string
	dir  *Dir
}

type Dir struct {
	path     string
	name     string
	children []Node
	parent   *Dir
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Name() string {
	return f.name
}

func (d *Dir) Path() string {
	return d.path
}

func (d *Dir) Name() string {
	return d.name
}
