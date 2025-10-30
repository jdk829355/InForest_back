package models

import (
	gen "github.com/jdk829355/InForest_back/protos/forest"
)

type Forest struct {
	UserId      string `json:"user_id"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Depth       int32  `json:"depth"`
	TotalTrees  int32  `json:"total_trees"`
	Root        *Tree  `json:"root"`
}

type Tree struct {
	Id       string  `json:"id"`
	Name     string  `json:"name"`
	Url      string  `json:"url"`
	Children []*Tree `json:"children"`
}

func (f *Forest) ToProto() *gen.Forest {
	if f == nil {
		return nil
	}
	return &gen.Forest{
		UserId:      f.UserId,
		Id:          f.Id,
		Name:        f.Name,
		Description: f.Description,
		Depth:       f.Depth,
		TotalTrees:  f.TotalTrees,
		Root:        f.Root.ToProto(),
	}
}

func (t *Tree) ToProto() *gen.Tree {
	if t == nil {
		return nil
	}
	children := make([]*gen.Tree, len(t.Children))
	for i, child := range t.Children {
		children[i] = child.ToProto()
	}
	return &gen.Tree{
		Id:       t.Id,
		Name:     t.Name,
		Url:      t.Url,
		Children: children,
	}
}
