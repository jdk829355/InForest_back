package forestservice

import (
	"context"
	"errors"

	"github.com/jdk829355/InForest_back/models"
	"github.com/jdk829355/InForest_back/protos/forest"
)

func (s *ForestService) GetForestsByUser(ctx context.Context, req *forest.GetForestsByUserRequest) (*forest.GetForestsByUserResponse, error) {
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}
	forests, err := s.Store.Neo4j.GetForestByUser(ctx, user_id.(string), req.GetIncludeChildren())
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
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}

	root := &models.Tree{
		Id:   req.GetRoot().GetId(),
		Name: req.GetRoot().GetName(),
		Url:  req.GetRoot().GetUrl(),
	}
	forestModel := &models.Forest{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		UserId:      user_id.(string),
		Root:        root,
	}
	if err := s.Store.Neo4j.CreateForest(ctx, forestModel, root); err != nil {
		return nil, err
	}
	s.Store.Supabase.CreateMemo(user_id.(string), root.Id, nil)
	return forestModel.ToProto(), nil
}

// 메모 적용 완료
func (s *ForestService) CreateTree(ctx context.Context, req *forest.CreateTreeRequest) (*forest.CreateTreeResponse, error) {
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}
	treeModel := &models.Tree{
		Id:   req.GetId(),
		Name: req.GetName(),
		Url:  req.GetUrl(),
	}
	id, err := s.Store.Neo4j.CreateTree(ctx, treeModel, req.GetParentId())
	if id == "" || err != nil {
		return nil, err
	}
	// 트리 생성 후 해당 메모 생성
	memo, err := s.Store.Supabase.CreateMemo(user_id.(string), id, nil)
	if err != nil {
		// 메모 생성 실패 시 트리도 삭제
		// TODO 다른 RPC 핸들러 호출해야겠음
		_, _ = s.Store.Neo4j.DeleteTree(ctx, id, true)
		return nil, err
	}
	return &forest.CreateTreeResponse{
		Tree: treeModel.ToProto(),
		Memo: memo.ToProto(),
	}, nil
}

func (s *ForestService) GetForest(ctx context.Context, req *forest.GetForestRequest) (*forest.GetForestResponse, error) {
	forestModel, err := s.Store.Neo4j.GetForest(ctx, req.GetForestId(), req.GetIncludeChildren())
	if err != nil {
		return nil, err
	}
	return &forest.GetForestResponse{
		Forest: forestModel.ToProto(),
	}, nil
}

func (s *ForestService) UpdateForest(ctx context.Context, req *forest.UpdateForestRequest) (*forest.Forest, error) {
	inputForestModel := &models.Forest{
		Id:          req.GetForestId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	}
	forestModel, err := s.Store.Neo4j.UpdateForest(ctx, inputForestModel)
	if err != nil {
		return nil, err
	}
	return forestModel.ToProto(), nil
}

func (s *ForestService) DeleteForest(ctx context.Context, req *forest.DeleteForestRequest) (*forest.DeleteForestResponse, error) {
	idsToDelete, err := s.Store.Neo4j.DeleteForest(ctx, req.GetForestId())
	if err != nil {
		return &forest.DeleteForestResponse{
			Success: false,
		}, err
	}
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}
	for _, treeID := range idsToDelete {
		s.DeleteTree(ctx, &forest.DeleteTreeRequest{TreeId: treeID, Cascade: false})
	}
	return &forest.DeleteForestResponse{
		Success: true,
	}, nil
}

func (s *ForestService) GetTree(ctx context.Context, req *forest.GetTreeRequest) (*forest.Tree, error) {
	tree, err := s.Store.Neo4j.GetTreeByID(ctx, req.GetTreeId(), req.GetIncludeChildren())
	if err != nil {
		return nil, err
	}
	return tree.ToProto(), nil
}

func (s *ForestService) UpdateTree(ctx context.Context, req *forest.UpdateTreeRequest) (*forest.Tree, error) {
	inputTreeModel := &models.Tree{
		Id:   req.GetTreeId(),
		Name: req.GetName(),
		Url:  req.GetUrl(),
	}
	treeModel, err := s.Store.Neo4j.UpdateTree(ctx, inputTreeModel)
	if err != nil {
		return nil, err
	}
	return treeModel.ToProto(), nil
}

// 트리 삭제 시 메모도 같이 삭제
func (s *ForestService) DeleteTree(ctx context.Context, req *forest.DeleteTreeRequest) (*forest.DeleteTreeResponse, error) {
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}
	deletedIds, err := s.Store.Neo4j.DeleteTree(ctx, req.GetTreeId(), req.GetCascade())
	if err != nil {
		return &forest.DeleteTreeResponse{
			Success: false,
		}, err
	}
	deletedMemos := map[string]models.Memo{}
	for _, treeID := range deletedIds {
		memo, err := s.Store.Supabase.DeleteMemo(user_id.(string), treeID)
		if err != nil {
			for _, m := range deletedMemos {
				// 롤백: 삭제된 메모 복구
				_, _ = s.Store.Supabase.CreateMemo(m.UserID, m.TreeID, map[string]interface{}{
					"content": m.Content,
					"version": m.Version,
				})
			}
			return nil, err
		} else {
			deletedMemos[memo.TreeID] = *memo
		}
	}
	return &forest.DeleteTreeResponse{
		Success: true,
	}, nil
}

func (s *ForestService) GetMemo(ctx context.Context, req *forest.GetMemoRequest) (*forest.Memo, error) {
	user_id := ctx.Value("user_id")
	if user_id == "" {
		return nil, errors.New("invalid user_id")
	}
	memo, err := s.Store.Supabase.GetMemo(user_id.(string), req.GetTreeId())
	if err != nil {
		return nil, err
	}
	return memo.ToProto(), nil
}
