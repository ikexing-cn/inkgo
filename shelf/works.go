package shelf

import (
	"errors"

	"writer/workspace"
)

// Describe reads what the shelf shows of the work at path.
func Describe(path string) (Book, error) {
	s, err := workspace.Open(path)
	if err != nil {
		return Book{}, err
	}
	defer s.Close()
	return DescribeOpen(s), nil
}

// DescribeOpen reads shelf information from a store the caller already has open.
func DescribeOpen(s *workspace.Store) Book {
	w := s.Work()
	b := Book{Path: s.Root(), Name: w.Name, Genre: w.Genre}
	counts, _ := s.Counts()
	for _, n := range counts {
		b.Words += n
	}
	return b
}

// Add reads and adds a work; the bool reports whether a new shelf entry was added.
func (s *Shelf) Add(path string) (Book, bool, error) {
	if !workspace.IsWork(path) {
		return Book{}, false, errors.New("该文件夹中没有作品（缺少 work.yaml）：" + path)
	}
	if s.Find(path) >= 0 {
		return Book{}, false, nil
	}
	b, err := Describe(path)
	if err != nil {
		return Book{}, false, err
	}
	s.Put(b)
	return b, true, nil
}

// RelocateWork validates and reads the destination work before Relocate,
// then refreshes its shelf information.
func (s *Shelf) RelocateWork(from, to string) error {
	if !workspace.IsWork(to) {
		return errors.New("该文件夹中没有作品（缺少 work.yaml）：" + to)
	}
	b, err := Describe(to)
	if err != nil {
		return err
	}
	s.Relocate(from, b.Path)
	s.Put(b)
	return nil
}
