package forestservice

import (
	"github.com/jdk829355/InForest_back/internal/store"
	"github.com/jdk829355/InForest_back/protos/forest"
)

type ForestService struct {
	forest.UnimplementedForestServiceServer
	Store *store.Store
}

func NewForestService(store *store.Store) *ForestService {
	return &ForestService{
		Store: store,
	}
}
