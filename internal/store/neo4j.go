package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jdk829355/InForest_back/config"
	"github.com/jdk829355/InForest_back/models"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/google/uuid"
)

type Neo4jStore struct {
	neo4jDriver neo4j.DriverWithContext
}

func NewNeo4jStore(driver neo4j.DriverWithContext) (*Neo4jStore, error) {
	return &Neo4jStore{
		neo4jDriver: driver,
	}, nil
}

func InitNeo4jStore(cfg *config.Config) (*neo4j.DriverWithContext, error) {
	ctx := context.Background()
	dbUri := cfg.Neo4jURI
	dbUser := cfg.Neo4jUsername
	dbPassword := cfg.Neo4jPassword
	driver, err := neo4j.NewDriverWithContext(
		dbUri,
		neo4j.BasicAuth(dbUser, dbPassword, ""))
	if err != nil {
		panic(err)
	}
	// neo4j 드라이버 연결 테스트
	for i := 0; i < 6; i++ {
		if err := driver.VerifyConnectivity(ctx); err == nil {
			return &driver, nil
		}
		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("neo4j driver connectivity verification failed: %w", err)
}

func (store *Neo4jStore) Close(ctx context.Context) error {
	return store.neo4jDriver.Close(ctx)
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
	tree.Children = nil // 자식 트리는 별도로 처리 필요
	return tree, nil
}

// 로직 시작

func (s *Neo4jStore) GetForestByUser(ctx context.Context, userID string) ([]*models.Forest, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)
	cypher := `MATCH (f:Forest {user_id: $user_id}) RETURN f.user_id AS user_id, f.id AS id, f.name AS name, f.description AS description, f.depth AS depth, f.total_trees AS total_trees`
	parameters := map[string]interface{}{
		"user_id": userID,
	}
	result, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}
	var forests []*models.Forest

	for result.Next(ctx) {
		record := result.Record()
		forest, err := s.parseForestRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse forest record: %w", err)
		}
		forests = append(forests, forest)
	}

	for i := range forests {
		cypher = `MATCH (f: Forest{id: $forestId})-[:derived]->(t: Tree) RETURN t.id AS id, t.name AS name, t.url AS url`
		parameters = map[string]interface{}{
			"forestId": forests[i].Id,
		}
		treeResult, err := session.Run(ctx, cypher, parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to run tree query: %w", err)
		}
		if treeResult.Next(ctx) {
			if record := treeResult.Record(); record != nil {
				rootTree, err := s.parseTreeRecord(record)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tree record: %w", err)
				}
				forests[i].Root = rootTree
			}
		}
	}

	return forests, nil
}

func (s *Neo4jStore) CreateForest(ctx context.Context, forest *models.Forest, root *models.Tree) error {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	forest.Id = uuid.New().String()
	forest.Depth = 1
	forest.TotalTrees = 1

	cypher := `CREATE (f:Forest {id: $id, name: $name, description: $description, depth: $depth, total_trees: $total_trees, user_id: $user_id})
	-[:derived]-> (t:Tree {id: $tree_id, name: $tree_name, url: $tree_url})`
	parameters := map[string]interface{}{
		"id":          forest.Id,
		"name":        forest.Name,
		"description": forest.Description,
		"depth":       forest.Depth,
		"total_trees": forest.TotalTrees,
		"user_id":     forest.UserId,
		"tree_id":     uuid.New().String(),
		"tree_name":   root.Name,
		"tree_url":    root.Url,
	}
	_, err := session.Run(ctx, cypher, parameters)
	return err
}

func (s *Neo4jStore) CreateTree(ctx context.Context, tree *models.Tree, parentID string) error {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	tree.Id = uuid.New().String()
	tree.Children = nil

	cypher := `MATCH (parent:Tree {id: $parent_id})
	CREATE (child:Tree {id: $id, name: $name, url: $url})
	CREATE (parent)-[:derived]->(child)`
	parameters := map[string]interface{}{
		"id":        tree.Id,
		"name":      tree.Name,
		"url":       tree.Url,
		"parent_id": parentID,
	}
	_, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return err
	}

	// 부모 트리의 숲 정보 업데이트
	cypher = `
	MATCH p = (f:Forest)-[:derived*]->(parent:Tree {id: $parent_id})
            WITH f, length(p) AS parent_depth
            SET f.total_trees = f.total_trees + 1
            SET f.depth = CASE 
                            WHEN (parent_depth + 1) > f.depth THEN (parent_depth + 1) 
                            ELSE f.depth 
                          END
`
	parameters = map[string]interface{}{
		"parent_id": parentID,
	}
	_, err = session.Run(ctx, cypher, parameters)
	if err != nil {
		return err
	}
	return nil
}
