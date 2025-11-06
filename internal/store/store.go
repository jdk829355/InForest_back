package store

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/supabase-community/supabase-go"
)

type Store struct {
	Neo4j    *Neo4jStore    // Neo4j 로직을 담당하는 구조체
	Supabase *SupabaseStore // Supabase 로직을 담당하는 구조체
}

func NewStore(neo4jDriver neo4j.DriverWithContext, supabaseClient *supabase.Client) *Store {
	neo4jStore, _ := NewNeo4jStore(neo4jDriver)
	supabaseStore, _ := NewSupabaseStore(supabaseClient)
	return &Store{
		Neo4j:    neo4jStore,
		Supabase: supabaseStore,
	}
}

func (s *Store) Close(ctx context.Context) {
	s.Neo4j.Close(ctx)
}
