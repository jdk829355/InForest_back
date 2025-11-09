package forestservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/jdk829355/InForest_back/models"
	"github.com/jdk829355/InForest_back/protos/forest"
	"github.com/redis/go-redis/v9"
)

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

func (s *ForestService) GetSummary(req *forest.GetSummaryRequest, stream forest.ForestService_GetSummaryServer) error {
	// 트리의 요약 조회 후 없으면 스트리밍 생성
	// 요약 생성 중간중간 진행상황 스트리밍
	// 요약이 있는 경우 바로 스트리밍으로 반환
	// 요약이 없는 경우 FastAPI 호출하여 요약 생성
	// 생성된 요약을 스트리밍으로 반환
	// 중복 요청 시 기존 요약 생성 작업에 합류하여 스트리밍으로 반환
	tree := &models.Tree{}
	tree, err := s.Store.Neo4j.GetTreeByID(stream.Context(), req.GetTreeId(), false)
	if err != nil {
		return errors.New("failed to get tree: " + err.Error())
	}
	if tree.Summary != "" {
		// 요약이 이미 존재하는 경우 바로 반환
		return stream.Send(&forest.GetSummaryResponse{
			Summary: tree.Summary,
			Status:  "completed",
		})
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	// 작업 중인 요약이 있는지 확인
	taskStatus, err := rdb.Get(stream.Context(), fmt.Sprintf("task_status:%s", tree.Id)).Result()
	// 작업 중인 요약이 없는 경우 새로 요약 생성 작업 시작
	if err == redis.Nil {
		body, err := json.Marshal(SummaryRequest{
			TreeID: tree.Id,
			Url:    tree.Url,
		})
		if err != nil {
			return err
		}
		newTaskreq, err := http.NewRequest(http.MethodPost, "http://ai_app:8000/task", bytes.NewBuffer(body))
		if err != nil {
			return err
		}
		newTaskreq.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(newTaskreq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to start summary task, status code: %d", resp.StatusCode)
		}
		if err := s.streamStatus(&stream, tree.Id, rdb); err != nil {
			return err
		}
		return nil
	}
	if err != nil && err != redis.Nil {
		return err
	}
	switch taskStatus {
	case "FAILED":
		// 이전 작업이 실패한 경우 새로 요약 생성 작업 시작
		body, err := json.Marshal(SummaryRequest{
			TreeID: tree.Id,
			Url:    tree.Url,
		})
		if err != nil {
			return err
		}
		newTaskreq, err := http.NewRequest(http.MethodPost, "http://ai_app:8000/task", bytes.NewBuffer(body))
		if err != nil {
			return err
		}
		newTaskreq.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(newTaskreq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to start summary task, status code: %d", resp.StatusCode)
		}
		if err := s.streamStatus(&stream, tree.Id, rdb); err != nil {
			return err
		}
		return nil
	case "PENDING":
		// 작업 중인 요약이 있는 경우 스트리밍으로 진행상황 반환
		if err := s.streamStatus(&stream, tree.Id, rdb); err != nil {
			return err
		}
		return nil
	case "COMPLETED":
		// 완료가 됐음에도 요약이 없는 경우 다시 요약 생성 작업 시작
		body, err := json.Marshal(SummaryRequest{
			TreeID: tree.Id,
			Url:    tree.Url,
		})
		if err != nil {
			return err
		}
		newTaskreq, err := http.NewRequest(http.MethodPost, "http://ai_app:8000/task", bytes.NewBuffer(body))
		if err != nil {
			return err
		}
		newTaskreq.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(newTaskreq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to start summary task, status code: %d", resp.StatusCode)
		}
		if err := s.streamStatus(&stream, tree.Id, rdb); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown task status: %s", taskStatus)
	}
}

func (s *ForestService) streamStatus(stream *forest.ForestService_GetSummaryServer, tree_id string, rdb *redis.Client) error {
	ctx := (*stream).Context()
	pubsub := rdb.Subscribe(ctx, tree_id)
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			return err
		}
		fmt.Println("payload: " + msg.Payload)

		switch msg.Payload {
		case "COMPLETED":
			tree := &models.Tree{}
			tree, err := s.Store.Neo4j.GetTreeByID(ctx, tree_id, false)
			if err != nil {
				return err
			}
			// 작업이 완료된 경우 스트리밍으로 결과 반환
			(*stream).Send(&forest.GetSummaryResponse{
				Summary: tree.Summary,
				Status:  "COMPLETED",
			})
			return nil
		case "IN_PROGRESS":
			// 작업이 진행 중인 경우 스트리밍으로 상태 반환
			(*stream).Send(&forest.GetSummaryResponse{
				Summary: "",
				Status:  "IN_PROGRESS",
			})
		case "FAILED":
			// 작업이 실패한 경우 스트리밍으로 상태 반환
			(*stream).Send(&forest.GetSummaryResponse{
				Summary: "",
				Status:  "FAILED",
			})
			return nil
		default:
			// 작업이 진행 중인 경우 스트리밍으로 상태 반환
			(*stream).Send(&forest.GetSummaryResponse{
				Summary: "",
				Status:  "PENDING",
			})
		}
	}
}

type SummaryRequest struct {
	TreeID string `json:"tree_id"`
	Url    string `json:"url"`
}
