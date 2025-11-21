/*
grpc 클라이언트를 다음 시나리오에 따라 실행
프로젝트 루트 하위의 protos/forest/forest.proto에 정의된 gRPC 서비스 사용
트리의 url은 https://python.org를 사용하며 모두 동일한 url을 사용
1. gRPC 서버에 연결 (host, port는 인자로 받음)
2. GetForestsByUser 메서드 호출
3. CreateForest 메서드 호출
4. GetForestsByUser 메서드 다시 호출하여 생성된 숲이 포함되었는지 확인
5. CreateTree 메서드 호출 (트리 여러 개 생성, 호출할 때 트리 id가 유효한지 확인할 것, ""이면 호출하지 말것)
6. GetSummary를 호출하여 웹사이트 요약정보 호출 (스트림 두 번 받고 재연결 하여 스트림이 이어지는지 테스트)
7. DeleteForest 메서드 호출
8. 끝
*/
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	pb "github.com/jdk829355/InForest_back/protos/forest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	// 1. gRPC 서버에 연결 (host, port는 인자로 받음)
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %s <host> <port> <tls:true|false> <token>", os.Args[0])
	}
	host := os.Args[1]
	port := os.Args[2]
	useTLS, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid TLS argument. Use 'true' or 'false': %v", err)
	}
	token := os.Args[4]
	address := fmt.Sprintf("%s:%s", host, port)

	// TLS 설정
	var opts []grpc.DialOption
	if useTLS {
		creds := credentials.NewClientTLSFromCert(nil, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
		log.Printf("Using TLS with system certificates")
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		log.Printf("Using insecure connection")
	}

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewForestServiceClient(conn)

	// metadata에 authorization 추가
	md := metadata.New(map[string]string{
		"authorization": "bearer " + token,
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	log.Printf("Connected to gRPC server at %s", address)
	log.Printf("Using authorization token: %s", token)

	// 2. GetForestsByUser 메서드 호출
	log.Println("\n=== Step 2: GetForestsByUser (초기 상태) ===")
	initialForests, err := client.GetForestsByUser(ctx, &pb.GetForestsByUserRequest{
		IncludeChildren: true,
	})
	if err != nil {
		log.Fatalf("GetForestsByUser failed: %v", err)
	}
	log.Printf("Initial forests count: %d", len(initialForests.Forests))
	for i, forest := range initialForests.Forests {
		log.Printf("  [%d] Forest: %s (ID: %s)", i+1, forest.Name, forest.Id)
	}

	// 3. CreateForest 메서드 호출
	log.Println("\n=== Step 3: CreateForest ===")
	newForest, err := client.CreateForest(ctx, &pb.CreateForestRequest{
		Name:        "Test Forest",
		Description: "This is a test forest created by gRPC client",
		Root: &pb.Tree{
			Name: "Root Tree",
			Url:  "https://python.org",
		},
	})
	if err != nil {
		log.Fatalf("CreateForest failed: %v", err)
	}
	log.Printf("Created forest: %s (ID: %s)", newForest.Name, newForest.Id)
	log.Printf("  Root tree ID: %s", newForest.Root.Id)
	forestId := newForest.Id
	rootTreeId := newForest.Root.Id

	// 4. GetForestsByUser 메서드 다시 호출하여 생성된 숲이 포함되었는지 확인
	log.Println("\n=== Step 4: GetForestsByUser (생성 후 확인) ===")
	updatedForests, err := client.GetForestsByUser(ctx, &pb.GetForestsByUserRequest{
		IncludeChildren: true,
	})
	if err != nil {
		log.Fatalf("GetForestsByUser failed: %v", err)
	}
	log.Printf("Updated forests count: %d", len(updatedForests.Forests))
	found := false
	for i, forest := range updatedForests.Forests {
		log.Printf("  [%d] Forest: %s (ID: %s)", i+1, forest.Name, forest.Id)
		if forest.Id == forestId {
			found = true
			log.Printf("    ✓ Found newly created forest!")
		}
	}
	if !found {
		log.Printf("    ⚠ Warning: Newly created forest not found in list")
	}

	// 5. CreateTree 메서드 호출 (트리 여러 개 생성, 호출할 때 트리 id가 유효한지 확인할 것, ""이면 호출하지 말것)
	log.Println("\n=== Step 5: CreateTree (여러 트리 생성) ===")
	trees := []struct {
		name     string
		parentId string
	}{
		{"Child Tree 1", rootTreeId},
		{"Child Tree 2", rootTreeId},
		{"Grandchild Tree 1", ""},
	}

	var childTree1Id string
	for i, treeData := range trees {
		parentId := treeData.parentId

		// i == 2인 경우 Grandchild의 부모를 Child Tree 1로 설정
		if i == 2 {
			if childTree1Id != "" {
				parentId = childTree1Id
			} else {
				log.Printf("  Skipping %s: Child Tree 1 ID not available", treeData.name)
				continue
			}
		}

		// parentId가 빈 문자열이면 호출하지 않음
		if parentId == "" {
			log.Printf("  Skipping %s: Parent ID is empty", treeData.name)
			continue
		}

		response, err := client.CreateTree(ctx, &pb.CreateTreeRequest{
			Name:     treeData.name,
			Url:      "https://python.org",
			ParentId: parentId,
		})
		if err != nil {
			log.Printf("  CreateTree failed for %s: %v", treeData.name, err)
			continue
		}
		log.Printf("  Created tree: %s (ID: %s)", response.Tree.Name, response.Tree.Id)

		// Child Tree 1의 ID 저장
		if i == 0 {
			childTree1Id = response.Tree.Id
		}
	} // 6. GetSummary를 호출하여 웹사이트 요약정보 호출 (스트림 두 번 받고 재연결 하여 스트림이 이어지는지 테스트)
	log.Println("\n=== Step 6: GetSummary (스트림 테스트) ===")

	// 첫 번째 스트림 호출
	log.Println("  첫 번째 스트림 시작...")
	stream1, err := client.GetSummary(ctx, &pb.GetSummaryRequest{
		TreeId: rootTreeId,
	})
	if err != nil {
		log.Printf("  GetSummary stream1 failed: %v", err)
	} else {
		count := 0
		for {
			response, err := stream1.Recv()
			if err == io.EOF {
				log.Printf("  첫 번째 스트림 종료 (총 %d개 메시지)", count)
				break
			}
			if err != nil {
				log.Printf("  Stream1 receive error: %v", err)
				break
			}
			count++
			log.Printf("    [Stream1-%d] Status: %s, Summary: %s", count, response.Status, response.Summary)
			if count >= 2 {
				log.Printf("  첫 번째 스트림에서 2개 메시지 수신 완료, 연결 끊김 시뮬레이션")
				break
			}
		}
	}

	// 재연결 대기
	log.Println("  재연결 대기 중 (2초)...")
	time.Sleep(2 * time.Second)

	// 두 번째 스트림 호출 (재연결) - 동일한 ID로 요청
	log.Println("  두 번째 스트림 시작 (재연결, 동일 ID)...")
	stream2, err := client.GetSummary(ctx, &pb.GetSummaryRequest{
		TreeId: rootTreeId,
	})
	if err != nil {
		log.Printf("  GetSummary stream2 failed: %v", err)
	} else {
		count := 0
		for {
			response, err := stream2.Recv()
			if err == io.EOF {
				log.Printf("  두 번째 스트림 종료 (총 %d개 메시지)", count)
				break
			}
			if err != nil {
				log.Printf("  Stream2 receive error: %v", err)
				break
			}
			count++
			log.Printf("    [Stream2-%d] Status: %s, Summary: %s", count, response.Status, response.Summary)

			// COMPLETED 상태를 받을 때까지 계속 수신
			if response.Status == "COMPLETED" {
				log.Printf("  두 번째 스트림에서 COMPLETED 수신, 종료")
				break
			}
		}
	}

	// 7. DeleteForest 메서드 호출
	log.Println("\n=== Step 7: DeleteForest ===")
	deleteResponse, err := client.DeleteForest(ctx, &pb.DeleteForestRequest{
		ForestId: forestId,
	})
	if err != nil {
		log.Fatalf("DeleteForest failed: %v", err)
	}
	log.Printf("Delete forest result: success=%v", deleteResponse.Success)

	// 8. 끝
	log.Println("\n=== Step 8: Complete ===")
	log.Println("All operations completed successfully!")
}
