package store

import (
	"context"
	"fmt"

	"github.com/jdk829355/InForest_back/models"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// 유틸함수
func getDerived(tree_from *models.Tree, ctx context.Context, session neo4j.SessionWithContext, s *Neo4jStore) error {
	cypher := `MATCH (parent:Tree {id: $parent_id})-[:derived]->(child:Tree) RETURN child.id AS id, child.name AS name, child.url AS url`
	parameters := map[string]interface{}{
		"parent_id": tree_from.Id,
	}
	derivedResult, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return err
	}
	for derivedResult.Next(ctx) {
		record := derivedResult.Record()
		child, err := s.parseTreeRecord(record)
		if err != nil {
			return err
		}
		tree_from.Children = append(tree_from.Children, child)
		getDerived(child, ctx, session, s)
	}
	return nil
}

func (s *Neo4jStore) parseForestRecord(record *neo4j.Record) (*models.Forest, error) {
	forest := &models.Forest{}
	var ok bool

	if forestData, exists := record.Get("id"); exists {
		forest.Id, ok = forestData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest id")
		}
	}
	if forestData, exists := record.Get("name"); exists {
		forest.Name, ok = forestData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest name")
		}
	}
	if forestData, exists := record.Get("description"); exists {
		forest.Description, ok = forestData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest description")
		}
	}
	if forestData, exists := record.Get("depth"); exists {
		depth, ok := forestData.(int64)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest depth")
		}
		forest.Depth = int32(depth)
	}
	if forestData, exists := record.Get("total_trees"); exists {
		totalTrees, ok := forestData.(int64)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest total_trees")
		}
		forest.TotalTrees = int32(totalTrees)
	}
	if forestData, exists := record.Get("user_id"); exists {
		forest.UserId, ok = forestData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for forest user_id")
		}
	}
	forest.Root = nil // 트리 구조는 별도로 처리 필요

	return forest, nil
}

func (s *Neo4jStore) parseTreeRecord(record *neo4j.Record) (*models.Tree, error) {
	tree := &models.Tree{}
	var ok bool

	if treeData, exists := record.Get("id"); exists {
		tree.Id, ok = treeData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for tree id")
		}
	}
	if treeData, exists := record.Get("name"); exists {
		tree.Name, ok = treeData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for tree name")
		}
	}
	if treeData, exists := record.Get("url"); exists {
		tree.Url, ok = treeData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for tree url")
		}
	}
	if treeData, exists := record.Get("summary"); exists {
		tree.Summary, ok = treeData.(string)
		if !ok {
			return nil, fmt.Errorf("invalid type for tree summary")
		}
	}
	tree.Children = nil // 자식 트리는 별도로 처리 필요
	return tree, nil
}
