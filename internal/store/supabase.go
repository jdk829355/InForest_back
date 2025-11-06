package store

import (
	"encoding/json"
	"fmt"

	"github.com/jdk829355/InForest_back/config"
	"github.com/jdk829355/InForest_back/models"
	"github.com/supabase-community/supabase-go"
)

type SupabaseStore struct {
	client *supabase.Client
}

func NewSupabaseStore(client *supabase.Client) (*SupabaseStore, error) {
	return &SupabaseStore{
		client: client,
	}, nil
}

func InitSupabaseStore(cfg *config.Config) (*supabase.Client, error) {
	client, err := supabase.NewClient(cfg.SUPABASE_URL, cfg.SUPABASE_KEY, &supabase.ClientOptions{})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// SupabaseStore 관련 로직
/*
TODO: 추가해야할 메서드
- 메모 추가: db에 메모 추가 (사실상 빈 메모 생성, 트리 생성 시 연동 목적)
- 메모 조회: db에서 메모 조회
- 메모 수정: 로컬 내용 동기화 (diff 적용, 버전 비교 후 업데이트)
	- 로컬 < db: force -> 덮어쓰고 버전 업데이트, normal: no action and success: false
	- 로컬 == db: normal -> no action, success: true
	- 로컬 > db: normal -> diff 적용하고 버전 업데이트, success: true
- 메모 삭제: db에서 메모 삭제 (트리 삭제 시 연동 목적)

- 요약 추가: db에 요약 추가
- 요약 조회: db에서 요약 조회
- 요약 수정: db에서 요약 수정
*/

func (s *SupabaseStore) CreateMemo(user_id string, tree_id string, options map[string]interface{}) (*models.Memo, error) {
	if options == nil {
		options = make(map[string]interface{})
		options["content"] = ""
		options["version"] = int32(0)
	}
	memo := &models.Memo{
		TreeID:  tree_id,
		UserID:  user_id,
		Content: options["content"].(string),
		Version: options["version"].(int32),
	}

	var mapData map[string]interface{}
	marshaledData, err := json.Marshal(memo)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshaledData, &mapData)
	if err != nil {
		return nil, err
	}

	_, _, err = s.client.From("memo").Insert(mapData, false, "", "", "").Execute()
	if err != nil {
		return nil, err
	}

	return memo, nil
}

func (s *SupabaseStore) GetMemo(user_id string, tree_id string) (*models.Memo, error) {
	var memos []models.Memo
	count, err := s.client.From("memo").Select("*", "exact", false).Eq("user_id", user_id).Eq("tree_id", tree_id).ExecuteTo(&memos)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("memo does not exist") // 메모가 없는 경우
	}
	return &memos[0], nil
}

func (s *SupabaseStore) DeleteMemo(user_id string, tree_id string) (*models.Memo, error) {
	var memos []models.Memo
	data, _, err := s.client.From("memo").Delete("", "").Eq("user_id", user_id).Eq("tree_id", tree_id).Execute()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &memos)
	if err != nil {
		return nil, err
	}
	return &memos[0], nil
}
