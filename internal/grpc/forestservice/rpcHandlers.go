package forestservice

import (
	"context"

	"github.com/jdk829355/InForest_back/models"
	"github.com/jdk829355/InForest_back/protos/forest"
)

func (s *ForestService) GetForestsByUser(ctx context.Context, req *forest.GetForestsByUserRequest) (*forest.GetForestsByUserResponse, error) {
	forests, err := s.Store.Neo4j.GetForestByUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	forestsProto := make([]*forest.Forest, len(forests))
	for i, f := range forests {
		forestsProto[i] = f.ToProto()
	}
	return &forest.GetForestsByUserResponse{
		Forests: forestsProto,
	}, nil
}

func (s *ForestService) CreateForest(ctx context.Context, req *forest.CreateForestRequest) (*forest.Forest, error) {
	root := &models.Tree{
		Id:   req.GetRoot().GetId(),
		Name: req.GetRoot().GetName(),
		Url:  req.GetRoot().GetUrl(),
	}
	forestModel := &models.Forest{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		UserId:      req.GetUserId(),
		Root:        root,
	}
	if err := s.Store.Neo4j.CreateForest(ctx, forestModel, root); err != nil {
		return nil, err
	}
	return forestModel.ToProto(), nil
}
func (s *ForestService) CreateTree(ctx context.Context, req *forest.CreateTreeRequest) (*forest.Tree, error) {
	treeModel := &models.Tree{
		Id:   req.GetId(),
		Name: req.GetName(),
		Url:  req.GetUrl(),
	}
	if err := s.Store.Neo4j.CreateTree(ctx, treeModel, req.GetParentId()); err != nil {
		return nil, err
	}
	return treeModel.ToProto(), nil
}
