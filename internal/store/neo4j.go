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
	fmt.Println("connecting to" + dbUri)
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

// 로직 시작

func (s *Neo4jStore) GetForestByUser(ctx context.Context, userID string, includeChildren bool) ([]*models.Forest, error) {
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
	if includeChildren {
		for _, forest := range forests {
			err = getDerived(forest.Root, ctx, session, s)
			if err != nil {
				return nil, err
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
	root.Id = uuid.New().String()
	root.Children = nil

	cypher := `CREATE (f:Forest {id: $id, name: $name, description: $description, depth: $depth, total_trees: $total_trees, user_id: $user_id})
	-[:derived]-> (t:Tree {id: $tree_id, name: $tree_name, url: $tree_url, summary: ""})`
	parameters := map[string]interface{}{
		"id":          forest.Id,
		"name":        forest.Name,
		"description": forest.Description,
		"depth":       forest.Depth,
		"total_trees": forest.TotalTrees,
		"user_id":     forest.UserId,
		"tree_id":     root.Id,
		"tree_name":   root.Name,
		"tree_url":    root.Url,
	}
	_, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return err
	}
	return nil
}

func (s *Neo4jStore) CreateTree(ctx context.Context, tree *models.Tree, parentID string) (string, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	tree.Id = uuid.New().String()
	tree.Children = nil

	cypher := `MATCH (parent:Tree {id: $parent_id})
	CREATE (child:Tree {id: $id, name: $name, url: $url, summary: ""})
	CREATE (parent)-[:derived]->(child)`
	parameters := map[string]interface{}{
		"id":        tree.Id,
		"name":      tree.Name,
		"url":       tree.Url,
		"parent_id": parentID,
	}
	_, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return "", err
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
		return "", err
	}
	return tree.Id, nil
}

func (s *Neo4jStore) GetForest(ctx context.Context, forestID string, include_children bool) (*models.Forest, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	cypher := `MATCH (f:Forest {id: $forest_id}) RETURN f.user_id AS user_id, f.id AS id, f.name AS name, f.description AS description, f.depth AS depth, f.total_trees AS total_trees`
	parameters := map[string]interface{}{
		"forest_id": forestID,
	}
	result, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}
	if result.Next(ctx) {
		record := result.Record()
		forest, err := s.parseForestRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse forest record: %w", err)
		}
		// 루트 트리의 하위 트리들 재귀적으로 가져오기
		cypher = `MATCH (f: Forest{id: $forestId})-[:derived]->(t: Tree) RETURN t.id AS id, t.name AS name, t.url AS url, t.summary AS summary`
		parameters = map[string]interface{}{
			"forestId": forest.Id,
		}
		treeResult, err := session.Run(ctx, cypher, parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to run tree query: %w", err)
		}

		var rootTree *models.Tree

		if treeResult.Next(ctx) {
			if record := treeResult.Record(); record != nil {
				rootTree, err = s.parseTreeRecord(record)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tree record: %w", err)
				}
				forest.Root = rootTree
			}
		}

		if include_children {
			err = getDerived(rootTree, ctx, session, s)
			if err != nil {
				return nil, err
			}
		}

		return forest, nil
	}
	return nil, fmt.Errorf("forest not found")
}

func (s *Neo4jStore) UpdateForest(ctx context.Context, forest *models.Forest) (models.Forest, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	if forest.Name+forest.Id == "" {
		return models.Forest{}, fmt.Errorf("invalid forest data")
	}

	cypher := `MATCH (f:Forest {id: $id})`
	parameters := map[string]interface{}{}
	if forest.Name != "" {
		cypher += ` SET f.name = $name`
		parameters["name"] = forest.Name
	}
	if forest.Description != "" {
		cypher += ` SET f.description = $description`
		parameters["description"] = forest.Description
	}
	parameters["id"] = forest.Id
	_, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return models.Forest{}, err
	}
	updatedForest, err := s.GetForest(ctx, forest.Id, false)
	if err == nil {
		return *updatedForest, nil
	}

	return models.Forest{}, fmt.Errorf("forest not found")
}

func (s *Neo4jStore) DeleteForest(ctx context.Context, forestID string) ([]string, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)
	cypher := `MATCH (f:Forest {id: $forest_id}) -[*]-> (t:Tree) return t.id AS id`
	parameters := map[string]interface{}{
		"forest_id": forestID,
	}
	result, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, err
	}
	idsToDelete := []string{}
	for result.Next(ctx) {
		record := result.Record()
		treeID, ok := record.Get("id")
		if !ok {
			return nil, fmt.Errorf("failed to get tree id from record: %v", record)
		}
		idsToDelete = append(idsToDelete, treeID.(string))
	}

	cypher = `MATCH (n:Forest {id: $forest_id}) OPTIONAL MATCH (n)-[*]->(d:Tree) DETACH DELETE n, d`
	parameters = map[string]interface{}{
		"forest_id": forestID,
	}
	_, err = session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, err
	}
	return idsToDelete, nil
}

func (s *Neo4jStore) UpdateTree(ctx context.Context, tree *models.Tree) (models.Tree, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	if tree.Name+tree.Id == "" {
		return models.Tree{}, fmt.Errorf("invalid tree data")
	}

	cypher := `MATCH (t:Tree {id: $id})`
	parameters := map[string]interface{}{}
	if tree.Name != "" {
		cypher += ` SET t.name = $name`
		parameters["name"] = tree.Name
	}
	if tree.Url != "" {
		cypher += ` SET t.url = $url`
		parameters["url"] = tree.Url
	}
	parameters["id"] = tree.Id
	_, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return models.Tree{}, err
	}
	updatedTree, err := s.GetTreeByID(ctx, tree.Id, false)
	if err == nil {
		return *updatedTree, nil
	}

	return models.Tree{}, fmt.Errorf("tree not found")
}

func (s *Neo4jStore) GetTreeByID(ctx context.Context, treeID string, includeChildren bool) (*models.Tree, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	cypher := `MATCH (t:Tree {id: $tree_id}) RETURN t.id AS id, t.name AS name, t.url AS url, t.summary AS summary`
	parameters := map[string]interface{}{
		"tree_id": treeID,
	}
	result, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}
	if result.Next(ctx) {
		record := result.Record()
		tree, err := s.parseTreeRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tree record: %w", err)
		}
		if includeChildren {
			err = getDerived(tree, ctx, session, s)
			if err != nil {
				return nil, err
			}
		}
		return tree, nil
	}
	return nil, fmt.Errorf("tree not found")
}

func (s *Neo4jStore) DeleteTree(ctx context.Context, treeID string, cascade bool) ([]string, error) {
	session := s.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)
	var cypher string
	var inspectCypher string
	var deletedId []string

	deletedId = append(deletedId, treeID)

	if cascade {
		inspectCypher = `MATCH (t:Tree {id: $tree_id})
		OPTIONAL MATCH (t)-[*]->(descendants)
		RETURN descendants.id as deletedId `
		parameters := map[string]interface{}{
			"tree_id": treeID,
		}

		resp, err := session.Run(ctx, inspectCypher, parameters)
		if err != nil {
			return nil, err
		}

		for resp.Next(ctx) {
			record := resp.Record()
			id, ok := record.Get("deletedId")
			if !ok {
				return nil, fmt.Errorf("failed to get deleted id from record: %v", record)
			}
			deletedId = append(deletedId, id.(string))
		}
	}

	// 숲에서 트리가 속한 숲 ID 조회
	var forestId string
	cypher = `MATCH (f: Forest)-[:derived*]->(t) WHERE t.id = $tree_id return f.id as forestId`
	parameters := map[string]interface{}{
		"tree_id": treeID,
	}

	resp, err := session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, err
	}

	if resp.Next(ctx) {
		record := resp.Record()
		id, ok := record.Get("forestId")
		if !ok {
			return nil, fmt.Errorf("failed to get forest id from record: %v", record)
		}
		forestId = id.(string)
	}

	if cascade {
		cypher = `MATCH (t:Tree {id: $tree_id})
		OPTIONAL MATCH (t)-[*]->(descendants)
		DETACH DELETE t, descendants
		return count(t) as deletedCount`
	} else {
		cypher = `match (target: Tree{id:$tree_id})<-[r:derived]-(parent:Tree)
		match (target)-[rd:derived]->(child:Tree)
		create (parent)-[nd:derived]->(child)
		detach delete (target)
		return count(target) as deletedCount
		`
	}
	parameters = map[string]interface{}{
		"tree_id": treeID,
	}

	resp, err = session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, err
	}

	if resp.Next(ctx) {
		deletedCount, ok := resp.Record().Get("deletedCount")
		if !ok || deletedCount.(int64) == 0 {
			return nil, fmt.Errorf("tree not found")
		}
	} else {
		return nil, fmt.Errorf("tree not found")
	}
	// 숲 정보 업데이트
	cypher = `
	MATCH (f:Forest {id: $forest_id})
	MATCH p = (f)-[:derived*]->(t:Tree)
	WITH f, max(length(p)) AS max_depth
	SET f.total_trees = f.total_trees - $deleted_trees, f.depth = max_depth
	RETURN f.id, f.depth, f.total_trees;
	`
	parameters = map[string]interface{}{
		"forest_id":     forestId,
		"deleted_trees": len(deletedId),
	}
	_, err = session.Run(ctx, cypher, parameters)
	if err != nil {
		return nil, err
	}

	return deletedId, nil
}
