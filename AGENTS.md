# 에러 처리
## Core Packages
+ Standard Library: `errors, fmt`를 기본으로 사용합니다.

## Naming Convention
+ 사전 정의된 에러 변수명은 반드시 Err 접두사를 사용합니다. (예: `var ErrNotFound = errors.New("not found")`)
+ 에러 메시지 내용은 소문자로 시작하며, 마침표(.)나 줄바꿈으로 끝나지 않도록 작성합니다.

## Return Value Structure
+ 에러는 항상 함수의 다중 반환값 중 가장 마지막에 위치해야 합니다.
+ 반환된 에러는 즉시 확인해야 하며, 특별한 주석 설명 없이 _를 사용하여 에러를 무시하는 것을 엄격히 금지합니다.

```Go
// 권장하는 방식
func processData() ([]byte, error) {
    data, err := fetchData()
    if err != nil {
        return nil, err
    }
    return data, nil
}
```

## Error Wrapping & Context
+ 하위 계층에서 발생한 에러를 상위 호출자로 전달할 때는 `fmt.Errorf`와 `%w` 동사를 사용하여 문맥을 반드시 추가합니다.
+ 이를 통해 에러가 발생한 작업이나 위치를 명확히 알 수 있도록 추적 가능한 체인을 구성합니다.

```Go
// 하위 에러를 래핑하여 문맥 추가
if err != nil {
	return fmt.Errorf("read reserver response body: %w", err)
}
```

## Error Checking (Is / As)
+ 래핑된 에러 객체를 비교할 때 == 연산자나 문자열 비교(예: `strings.Contains`) 사용을 금지합니다.
+ 특정 에러 값인지 확인할 때는 반드시 `errors.Is`를 사용합니다.
+ 특정 에러 타입으로 변환하여 내부 필드에 접근할 때는 `errors.As`를 사용합니다.

```Go
// errors.Is 사용 (값 비교)
if errors.Is(err, ErrWebhookRequestFail) {
// 재시도 로직 수행
}

// errors.As 사용 (타입 캐스팅 및 내부 데이터 접근)
var valErr *ValidationError
if errors.As(err, &valErr) {
    fmt.Printf("Validation failed on field: %s\n", valErr.Field)
}
```

## Logging & Return Rule
+ 로깅은 `slog`를 사용하여 로깅합니다.
+ 에러를 로깅하는 것과 반환하는 것을 동시에 수행하지 않습니다.
+ 에러는 최상위 계층(예: HTTP 핸들러, main 함수)이나 조치를 취할 수 있는 지점에서 한 번만 로깅해야 계층별 중복 로그 생성을 방지할 수 있습니다.
+ 로그 메시지(타입)는 코드 내에 문자열로 직접 입력하지 않습니다. 반드시 log.go와 같은 전용 파일에 상수로 정의하여 일관되게 사용해야 합니다.

```go
package logger

const (
    LogProcessFailed = "failed to process"
    LogAuthFailed    = "authentication failed"
    LogDBQueryError  = "database query error"
)
```

```Go
// 금지하는 방식 (로그 중복 발생 및 하드코딩)
if err != nil {
    slog.Error("failed to process", "error", err) // 하드코딩된 문자열
    return err                                    // 로깅과 반환 동시 수행
}
// ✅ 권장하는 방식 2: 최상위 계층 (main 또는 HTTP 핸들러에서 로깅)
if err != nil {
    // 사전에 정의된 상수를 사용하여 로깅합니다.
	slog.Info(
	    LogProcessFailed.String(),
        "status_code", uploadResponse.StatusCode,
        "message", uploadResponse.Message,
        ...
    )
    return fmt.Errorf("process phase failed: %w", err)
}
```
## Panic & Recover
+ panic은 프로그램이 더 이상 실행을 유지할 수 없는 치명적인 상태(예: 초기 설정 파일 누락, 개발자의 명백한 논리적 오류)에서만 제한적으로 사용합니다.

# 디렉토리
## `/cmd` (Main Applications)
+ 애플리케이션의 진입점인 `main.go` 파일이 위치하는 디렉토리입니다.
+ 이곳의 코드는 최소화해야 하며, 복잡한 비즈니스 로직을 직접 작성하는 것을 엄격히 금지합니다.
+ 환경 변수 로드, 의존성 주입, 시스템 컴포넌트 초기화 및 서버/워커 실행 역할만 수행합니다.
+ 프로젝트 내에 여러 실행 파일(예: API 서버, 크롤러 데몬 등)이 존재할 수 있으므로, 하위 디렉토리로 애플리케이션 이름을 명시합니다. (예: `cmd/api-server/main.go`, `cmd/crawler/main.go`)

## `/api` (API Protocol Definitions)
+ OpenAPI(Swagger) 명세서, JSON 스키마, gRPC용 Protocol Buffers(.proto 파일) 등 API 계약 및 스펙을 정의하는 파일들을 보관합니다.

## `/scripts` (Build and Ops Scripts)
+ 빌드, 배포, 데이터베이스 마이그레이션, 테스트 자동화 등을 수행하는 쉘 스크립트(.sh)나 유틸리티 스크립트를 관리합니다.

## `/test` (Integration Tests and Fixtures)
+ 일반적인 단위 테스트(Unit Test, *_test.go) 코드는 테스트할 대상 코드가 위치한 패키지 폴더 안에 동일하게 위치시키는 것이 Go의 기본 규칙입니다.
+ 반면, 이 `/test` 디렉토리에는 여러 패키지를 통합적으로 검증하는 E2E 테스트 코드나 통합 테스트 환경을 구축하는 스크립트, 그리고 방대한 모의 데이터(Test Fixtures) 등을 분리하여 보관합니다.


# 파일 분리
## 에러
+ `errors.go`로 분리: 패키지 내 여러 파일에서 공통으로 재사용되는 에러 변수(예: `ErrNotFound`, `ErrInvalidInput`)나 커스텀 에러 구조체는 `errors.go`라는 단일 파일에 모아서 정의합니다. 이를 통해 패키지가 어떤 에러들을 반환하는지 한눈에 파악할 수 있습니다.
+ 도메인 파일 내 정의: 특정 비즈니스 로직(예: 웹훅 전송)에만 강하게 종속된 에러는 별도로 분리하지 않고, 해당 기능이 구현된 파일(예: webhook.go)의 최상단(import 블록 바로 아래)에 위치시킵니다.

## 상수
+ `consts.go`로 분리: 패키지 전역에서 광범위하게 참조하는 환경 설정 성격의 상수(예: 기본 타임아웃, 최대 제한 크기, API 엔드포인트 도메인 등)는 `consts.go` 파일에 모아 관리합니다.
+ 파일 내 const 블록 묶음: 특정 기능이나 단일 파일 내부에서만 사용하는 상수는 분리하지 않습니다. 대신 해당 파일의 상단에 `const (...)` 블록을 생성하여 관련된 상수들을 논리적으로 묶어둡니다.

## 테스트
+ 테스트 코드는 대상이 되는 파일과 정확히 동일한 경로에 위치하며, 파일명 끝에 `_test.go`를 붙여야 합니다. (예: `alarm.go`의 테스트는 `alarm_test.go`)
+ 테스트 커버리지는 80%를 이상으로 해야 합니다.

# 주석

## Core Tools
+ `go doc` 및 `go install golang.org/x/tools/cmd/godoc@latest`를 통해 추출 가능한 형식을 표준으로 사용합니다.
+ `gopls`가 해석하여 툴팁으로 제공하는 형식을 준수해야 합니다.

## Naming & Syntax Convention
+ 모든 공개 요소의 주석은 반드시 해당 식별자의 이름으로 시작합니다.
+ 주석은 마침표로 끝나는 완전한 문장으로 작성합니다.
+ 첫 번째 문장은 해당 요소의 기능을 완벽하게 요약해야 하며, 문서 목록의 요약문으로 사용됩니다.
+ 주석은 `한글`로 작성합니다.
+ `consts.go`, `errors.go` 에는 주석을 달지 않습니다.
+ `struct` 필드에는 동시성 관련된 필드가 아니면 주석을 달지 않습니다.

## Formatting 
+ 불렛 포인트(`*`, `-`, `+`) 또는 숫자(`1.`)를 사용하며, 목록 전후에는 반드시 빈 줄을 삽입합니다.
+ `[텍스트](URL)` 또는 `[식별자]` 형식을 사용하여 외부 문서나 내부 패키지 요소를 참조합니다.
+ 주석 내에서 탭(`\t`) 또는 공백 4칸으로 들여쓰기하여 코드 예시를 작성합니다.

```go
// Service는 네트워크 사이드카의 핵심 제어 로직을 담당합니다.
//
// 이 구조체는 모든 메서드에서 Thread-safe를 보장합니다.
// 자세한 내용은 [Config] 구조체를 참조하십시오.
type Service struct {
    // ID는 서비스의 고유 식별자입니다.
    ID string
}

// Run은 사이드카 서비스를 시작합니다.
//
// 지정된 포트에서 수신 대기하며 에러 발생 시 즉시 반환합니다.
func (s *Service) Run(port int) error {
    // 구현 로직
    return nil
}