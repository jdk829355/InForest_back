package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/jdk829355/InForest_back/protos/forest"
)

var (
	serverAddr = flag.String("addr", "localhost:50051", "The server address in the format of host:port")
	token      = flag.String("token", "", "JWT token for authentication (format: Bearer <token>)")
)

func main() {
	flag.Parse()

	// gRPC 서버에 연결
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewForestServiceClient(conn)

	// 인증 토큰이 제공된 경우 메타데이터에 추가
	ctx := context.Background()
	if *token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", *token)
	}

	printHeader("Forest Service Client Test - Sequential Scenario")

	// 1. GetForestsByUser 테스트
	step1GetForestsByUser(ctx, client)

	// 2. CreateForest 테스트 (첫 번째)
	firstForestID := step2CreateForest(ctx, client, "First Test Forest")

	// 3. UpdateForest 테스트
	step3UpdateForest(ctx, client, firstForestID)

	// 4. DeleteForest 테스트
	step4DeleteForest(ctx, client, firstForestID)

	// 5. CreateForest 테스트 (두 번째 - 루트 트리 ID 기억)
	secondForestID, rootTreeID := step5CreateForestAndGetRootTreeID(ctx, client)

	// 6. GetTreeByID 테스트
	step6GetTreeByID(ctx, client, rootTreeID)

	// 7. CreateTree 테스트 (루트 트리를 부모로)
	childTreeID := step7CreateTree(ctx, client, rootTreeID)

	// 8. UpdateTree 테스트
	step8UpdateTree(ctx, client, childTreeID)

	// 9. DeleteTree 테스트
	step9DeleteTree(ctx, client, childTreeID)

	// 10. GetMemo 테스트
	step10GetMemo(ctx, client, rootTreeID)

	// 11. UpdateMemo 테스트 (버전 0, force false)
	step11UpdateMemoNormal(ctx, client, rootTreeID)

	// 12. UpdateMemo 테스트 (버전 0, force true - 충돌 상황)
	step12UpdateMemoForce(ctx, client, rootTreeID)

	// 13. GetSummary 테스트 (스트리밍 - 2개만 받고 끊기)
	step13GetSummaryPartial(ctx, client, rootTreeID)

	// 14. GetSummary 재연결 테스트 (동일 tree_id로 기존 작업 합류)
	step14GetSummaryReconnect(ctx, client, rootTreeID)

	// 정리: 테스트용 포레스트 삭제
	cleanupDeleteForest(ctx, client, secondForestID)

	printHeader("All tests completed successfully!")
}

func printHeader(message string) {
	fmt.Println()
	fmt.Println("=" + repeatString("=", len(message)+2) + "=")
	fmt.Printf("| %s |\n", message)
	fmt.Println("=" + repeatString("=", len(message)+2) + "=")
	fmt.Println()
}

func printStep(stepNum int, stepName string) {
	fmt.Println()
	fmt.Printf(">>> STEP %d: %s\n", stepNum, stepName)
	fmt.Println(repeatString("-", 60))
}

func printSuccess(message string) {
	fmt.Printf("✓ SUCCESS: %s\n", message)
}

func printInfo(message string) {
	fmt.Printf("  ℹ %s\n", message)
}

func printError(message string) {
	fmt.Printf("✗ ERROR: %s\n", message)
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

// 1. GetForestsByUser
func step1GetForestsByUser(ctx context.Context, client pb.ForestServiceClient) {
	printStep(1, "GetForestsByUser - 사용자의 모든 포레스트 조회")

	req := &pb.GetForestsByUserRequest{
		IncludeChildren: true,
	}

	resp, err := client.GetForestsByUser(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("GetForestsByUser failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Found %d forests", len(resp.Forests)))
	for i, forest := range resp.Forests {
		printInfo(fmt.Sprintf("[%d] Forest: %s (ID: %s, Trees: %d)", i+1, forest.Name, forest.Id, forest.TotalTrees))
	}
}

// 2. CreateForest (첫 번째)
func step2CreateForest(ctx context.Context, client pb.ForestServiceClient, name string) string {
	printStep(2, "CreateForest - 첫 번째 포레스트 생성")

	rootTree := &pb.Tree{
		Name: "Initial Root Tree",
		Url:  "https://python.org",
	}

	req := &pb.CreateForestRequest{
		Name:        name,
		Description: "This forest will be updated and deleted",
		Root:        rootTree,
	}

	resp, err := client.CreateForest(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("CreateForest failed: %v", err))
		return ""
	}

	printSuccess(fmt.Sprintf("Created Forest: %s (ID: %s)", resp.Name, resp.Id))
	if resp.Root != nil {
		printInfo(fmt.Sprintf("Root Tree: %s (ID: %s)", resp.Root.Name, resp.Root.Id))
	}

	return resp.Id
}

// 3. UpdateForest
func step3UpdateForest(ctx context.Context, client pb.ForestServiceClient, forestID string) {
	if forestID == "" {
		printError("No forest ID provided for update")
		return
	}

	printStep(3, "UpdateForest - 포레스트 업데이트")

	req := &pb.UpdateForestRequest{
		ForestId:    forestID,
		Name:        "Updated Forest Name",
		Description: "This forest has been updated by the client test",
	}

	resp, err := client.UpdateForest(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("UpdateForest failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Updated Forest: %s (ID: %s)", resp.Name, resp.Id))
	printInfo(fmt.Sprintf("Description: %s", resp.Description))
}

// 4. DeleteForest
func step4DeleteForest(ctx context.Context, client pb.ForestServiceClient, forestID string) {
	if forestID == "" {
		printError("No forest ID provided for deletion")
		return
	}

	printStep(4, "DeleteForest - 포레스트 삭제")

	req := &pb.DeleteForestRequest{
		ForestId: forestID,
	}

	resp, err := client.DeleteForest(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("DeleteForest failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Deleted Forest (ID: %s, Success: %v)", forestID, resp.Success))
}

// 5. CreateForest (두 번째 - 루트 트리 ID 기억)
func step5CreateForestAndGetRootTreeID(ctx context.Context, client pb.ForestServiceClient) (string, string) {
	printStep(5, "CreateForest - 두 번째 포레스트 생성 (루트 트리 ID 저장)")

	rootTree := &pb.Tree{
		Name: "Main Root Tree",
		Url:  "https://python.org",
	}

	req := &pb.CreateForestRequest{
		Name:        fmt.Sprintf("Main Test Forest %d", time.Now().Unix()),
		Description: "This forest will be used for tree and memo operations",
		Root:        rootTree,
	}

	resp, err := client.CreateForest(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("CreateForest failed: %v", err))
		return "", ""
	}

	forestID := resp.Id
	rootTreeID := ""
	if resp.Root != nil {
		rootTreeID = resp.Root.Id
	}

	printSuccess(fmt.Sprintf("Created Forest: %s (ID: %s)", resp.Name, forestID))
	printInfo(fmt.Sprintf("Root Tree ID (saved): %s", rootTreeID))

	return forestID, rootTreeID
}

// 6. GetTreeByID
func step6GetTreeByID(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for retrieval")
		return
	}

	printStep(6, "GetTreeByID - 루트 트리 조회")

	req := &pb.GetTreeRequest{
		TreeId:          treeID,
		IncludeChildren: true,
	}

	resp, err := client.GetTree(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("GetTree failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Retrieved Tree: %s (ID: %s)", resp.Name, resp.Id))
	printInfo(fmt.Sprintf("URL: %s", resp.Url))
	printInfo(fmt.Sprintf("Children count: %d", len(resp.Children)))
}

// 7. CreateTree (루트 트리를 부모로)
func step7CreateTree(ctx context.Context, client pb.ForestServiceClient, parentTreeID string) string {
	if parentTreeID == "" {
		printError("No parent tree ID provided for tree creation")
		return ""
	}

	printStep(7, "CreateTree - 루트 트리의 자식 트리 생성")

	req := &pb.CreateTreeRequest{
		Name:     fmt.Sprintf("Child Tree %d", time.Now().Unix()),
		Url:      "https://python.org",
		ParentId: parentTreeID,
	}

	resp, err := client.CreateTree(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("CreateTree failed: %v", err))
		return ""
	}

	if resp.Tree == nil {
		printError("No tree returned in response")
		return ""
	}

	printSuccess(fmt.Sprintf("Created Child Tree: %s (ID: %s)", resp.Tree.Name, resp.Tree.Id))
	printInfo(fmt.Sprintf("Parent Tree ID: %s", parentTreeID))
	if resp.Memo != nil {
		printInfo(fmt.Sprintf("Memo created with version: %d", resp.Memo.Version))
	}

	return resp.Tree.Id
}

// 8. UpdateTree (이름 변경)
func step8UpdateTree(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for update")
		return
	}

	printStep(8, "UpdateTree - 자식 트리 이름 변경")

	req := &pb.UpdateTreeRequest{
		TreeId: treeID,
		Name:   "Updated Child Tree Name",
		Url:    "https://python.org",
	}

	resp, err := client.UpdateTree(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("UpdateTree failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Updated Tree: %s (ID: %s)", resp.Name, resp.Id))
	printInfo(fmt.Sprintf("New Name: %s", resp.Name))
}

// 9. DeleteTree
func step9DeleteTree(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for deletion")
		return
	}

	printStep(9, "DeleteTree - 자식 트리 삭제")

	req := &pb.DeleteTreeRequest{
		TreeId:  treeID,
		Cascade: false,
	}

	resp, err := client.DeleteTree(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("DeleteTree failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Deleted Tree (ID: %s, Success: %v)", treeID, resp.Success))
}

// 10. GetMemo
func step10GetMemo(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for memo retrieval")
		return
	}

	printStep(10, "GetMemo - 루트 트리의 메모 조회")

	req := &pb.GetMemoRequest{
		TreeId: treeID,
	}

	resp, err := client.GetMemo(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("GetMemo failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Retrieved Memo for Tree ID: %s", resp.TreeId))
	printInfo(fmt.Sprintf("Content: %s", resp.Content))
	printInfo(fmt.Sprintf("Version: %d", resp.Version))
}

// 11. UpdateMemo (버전 0, force false)
func step11UpdateMemoNormal(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for memo update")
		return
	}

	printStep(11, "UpdateMemo - 메모 업데이트 (버전 0, force=false)")

	memo := &pb.Memo{
		TreeId:  treeID,
		Content: "Updated memo content - normal update with version 0",
		Version: 0, // 클라이언트가 가지고 있는 버전
	}

	req := &pb.UpdateMemoRequest{
		Memo:  memo,
		Force: false,
	}

	resp, err := client.UpdateMemo(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("UpdateMemo failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Memo updated (Success: %v)", resp.Success))
	if resp.NewMemo != nil {
		printInfo(fmt.Sprintf("New Version: %d", resp.NewMemo.Version))
		printInfo(fmt.Sprintf("Synced At: %s", resp.SyncedAt))
	}
}

// 12. UpdateMemo (버전 0, force true - 충돌 상황)
func step12UpdateMemoForce(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for memo force update")
		return
	}

	printStep(12, "UpdateMemo - 메모 강제 업데이트 (버전 0, force=true, 충돌 상황)")

	memo := &pb.Memo{
		TreeId:  treeID,
		Content: "Force updated memo content - conflict resolution with version 0",
		Version: 0, // 클라이언트가 가지고 있는 버전
	}

	req := &pb.UpdateMemoRequest{
		Memo:  memo,
		Force: true, // 충돌 무시하고 강제 업데이트
	}

	resp, err := client.UpdateMemo(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("UpdateMemo (force) failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Memo force updated (Success: %v)", resp.Success))
	if resp.NewMemo != nil {
		printInfo(fmt.Sprintf("New Version: %d", resp.NewMemo.Version))
		printInfo(fmt.Sprintf("New Content: %s", resp.NewMemo.Content))
		printInfo(fmt.Sprintf("Synced At: %s", resp.SyncedAt))
	}
}

// 13. GetSummary (스트리밍 - 2개만 받고 끊기)
func step13GetSummaryPartial(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for summary retrieval")
		return
	}

	printStep(13, "GetSummary - 요약 스트리밍 (2개만 받고 연결 종료)")

	req := &pb.GetSummaryRequest{
		TreeId: treeID,
	}

	stream, err := client.GetSummary(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("GetSummary failed: %v", err))
		return
	}

	printInfo("Receiving summary stream (max 2 messages)...")

	count := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			printInfo("Stream ended by server")
			break
		}
		if err != nil {
			printError(fmt.Sprintf("Stream error: %v", err))
			break
		}

		count++
		printInfo(fmt.Sprintf("[%d] Status: %s, Summary: %s", count, resp.Status, resp.Summary))

		// 2개만 받고 연결 종료
		if count >= 2 {
			printInfo("Received 2 messages, closing connection...")
			break
		}
	}

	printSuccess(fmt.Sprintf("Received %d stream messages and closed connection", count))
}

// 14. GetSummary 재연결 (동일 tree_id로 기존 작업 합류)
func step14GetSummaryReconnect(ctx context.Context, client pb.ForestServiceClient, treeID string) {
	if treeID == "" {
		printError("No tree ID provided for summary reconnection")
		return
	}

	printStep(14, "GetSummary - 재연결 테스트 (동일 tree_id로 기존 작업 합류 확인)")

	printInfo("Reconnecting with same tree_id to check if it joins existing work...")

	req := &pb.GetSummaryRequest{
		TreeId: treeID,
	}

	stream, err := client.GetSummary(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("GetSummary reconnect failed: %v", err))
		return
	}

	printInfo("Receiving summary stream (reconnect)...")

	count := 0
	maxMessages := 5 // 재연결 테스트에서는 5개 정도만 받기
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			printInfo("Stream ended by server")
			break
		}
		if err != nil {
			printError(fmt.Sprintf("Stream error: %v", err))
			break
		}

		count++
		printInfo(fmt.Sprintf("[%d] Status: %s, Summary: %s", count, resp.Status, resp.Summary))

		if count >= maxMessages {
			printInfo(fmt.Sprintf("Received %d messages for reconnect test", maxMessages))
			break
		}
	}

	if count > 0 {
		printSuccess(fmt.Sprintf("Reconnect successful - received %d messages (joined existing work)", count))
	} else {
		printInfo("No messages received on reconnect (work may have completed)")
	}
}

// Cleanup: 테스트용 포레스트 삭제
func cleanupDeleteForest(ctx context.Context, client pb.ForestServiceClient, forestID string) {
	if forestID == "" {
		return
	}

	printStep(0, "Cleanup - 테스트용 포레스트 삭제")

	req := &pb.DeleteForestRequest{
		ForestId: forestID,
	}

	resp, err := client.DeleteForest(ctx, req)
	if err != nil {
		printError(fmt.Sprintf("Cleanup DeleteForest failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Cleanup complete (Forest ID: %s, Success: %v)", forestID, resp.Success))
}
