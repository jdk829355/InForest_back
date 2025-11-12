package forestservice

import (
	"context"
	"errors"
	"time"

	"github.com/jdk829355/InForest_back/protos/forest"
)

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

func (s *ForestService) UpdateMemo(ctx context.Context, req *forest.UpdateMemoRequest) (*forest.UpdateMemoResponse, error) {
	// 1. 버전 비교
	// 2. 요청에 있는 base_version과 현재 버전이 같은지 확인
	// 2-1. 같으면 업데이트 하고 success 반환
	// 2-2. base < current: 누군가가 중간에 업데이트를 함 -> false 반환
	// 2-3. base > current: 말도 안되는 상황 -> 에러 반환
	// 필요한 db 함수
	// - GetMemo
	// - UpdateMemo
	user_id := ctx.Value("user_id")

	memo, err := s.Store.Supabase.GetMemo(user_id.(string), req.GetMemo().GetTreeId())
	if err != nil {
		return nil, err
	}

	// 강제로 업데이트 하는 경우 (덮어쓰기)
	if req.GetForce() {
		newMemo, err := s.Store.Supabase.UpdateMemo(user_id.(string), req.GetMemo().GetTreeId(), req.GetMemo().GetContent(), memo.Version+1)
		if err != nil {
			return nil, err
		}
		return &forest.UpdateMemoResponse{
			Success:  true,
			NewMemo:  newMemo.ToProto(),
			SyncedAt: time.Now().Format(time.RFC3339),
		}, nil
	}

	if req.GetMemo().GetVersion() < memo.Version {
		// 2-2
		return &forest.UpdateMemoResponse{
			Success: false,
		}, errors.New("conflict: other version exists")
	} else if req.GetMemo().GetVersion() > memo.Version {
		// 2-3
		return nil, errors.New("invalid version")
	} else {
		newMemo, err := s.Store.Supabase.UpdateMemo(user_id.(string), req.GetMemo().GetTreeId(), req.GetMemo().GetContent(), req.GetMemo().GetVersion()+1)
		if err != nil {
			return nil, err
		}
		return &forest.UpdateMemoResponse{
			Success:  true,
			NewMemo:  newMemo.ToProto(),
			SyncedAt: time.Now().Format(time.RFC3339),
		}, nil
	}
}
