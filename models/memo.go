package models

import "github.com/jdk829355/InForest_back/protos/forest"

type Memo struct {
	TreeID  string `json:"tree_id"`
	UserID  string `json:"user_id"`
	Content string `json:"content"`
	Version int32  `json:"version"`
}

func (m *Memo) ToProto() *forest.Memo {
	return &forest.Memo{
		TreeId:  m.TreeID,
		Content: m.Content,
		Version: m.Version,
	}
}
