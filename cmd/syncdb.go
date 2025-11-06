package main

import (
	"context"

	"github.com/jdk829355/InForest_back/config"
	"github.com/jdk829355/InForest_back/internal/store"
)

// supabase와 neo4j를 동기화하는 별도의 커맨드라인 툴입니다.
// 예를 들어, supabase에 메모가 없는데 neo4j에 트리가 있는 경우,
// supabase에 빈 메모를 생성하는 등의 작업을 수행할 수 있습니다.

func main() {
	ctx := context.Background()
	// supabase와 neo4j를 동기화하는 작업을 수행합니다.
	cfg, _ := config.LoadConfig()
	neo4jDriver, err := store.InitNeo4jStore(cfg)
	if err != nil {
		panic(err)
	}
	supa_client, err := store.InitSupabaseStore(cfg)
	if err != nil {
		panic(err)
	}
	store := store.NewStore(*neo4jDriver, supa_client)
	defer store.Close(ctx)
	// 전체 트리 목록 조회
	// 트리마다 supabase에 메모가 있는지 확인
	// 없으면 빈 메모 생성

}
