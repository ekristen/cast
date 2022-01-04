package cache

import (
	"os"
	"path/filepath"
)

type Cache struct {
	Path string
}

func New(path string) (*Cache, error) {
	c := &Cache{
		Path: path,
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Cache) NewSubpath(path string) (*Cache, error) {
	nc := &Cache{
		Path: filepath.Join(c.Path, path),
	}

	if err := os.MkdirAll(nc.Path, 0755); err != nil {
		return nil, err
	}

	return nc, nil
}

func (c *Cache) GetPath() string {
	return c.Path
}
