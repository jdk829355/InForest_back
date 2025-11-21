# 프로토버프 파일을 컴파일하여 Go 소스 코드를 생성합니다.
# `protoc` 명령어를 사용하며, protoc-gen-go와 protoc-gen-go-grpc 플러그인을 활용합니다.
# .proto 파일이 있는 디렉토리와 생성될 코드가 위치할 디렉토리를 지정해야 합니다.
gen:
	protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	./protos/forest/forest.proto

# 생성된 Go 소스 파일(*_grpc.pb.go, *_pb.go) 및 기타 생성된 파일을 정리합니다.
clean:
	rm -f ./protos/forest/*_grpc.pb.go ./protos/forest/*_pb.go

# 서버 애플리케이션을 실행합니다.
# 'go run' 명령어를 사용하여 main 패키지의 main.go 파일을 실행합니다.
# 환경변수 로드 후 값 echo 출력
run_server: gen
	go run cmd/server/main.go

run_db:
	docker run -v inforest_back_db_data:/data -p 7474:7474 -p 7687:7687 --env-file ./config/.dbenv -d --rm --name db neo4j:latest

build:
	go build -o bin/server cmd/server/main.go