# Sidecargo

Sidecargo는 마이크로서비스 간 내부 통신 앞단에서 공통 정책을 처리하는 경량 sidecar입니다.

각 서비스가 인증, 로깅, 메트릭, 재시도, 요청 추적 같은 횡단 관심사를 중복 구현하지 않도록, 서비스 옆에 붙는 작은 프록시 계층으로 동작하는 것을 목표로 합니다.

이 프로젝트는 "서비스 간 호출을 안전하고 관측 가능하게 만드는 보조 런타임"에 가깝습니다. 거대한 API Gateway나 Service Mesh 전체를 대체하려는 것이 아니라, 특정 서비스 주변에서 필요한 정책을 가볍게 적용하는 데 초점을 둡니다.

## 역할

Sidecargo는 다음과 같은 역할을 수행합니다.

- 서비스 간 요청에 대한 JWT 검증
- 요청 단위 식별자 생성 및 전파
- 접근 로그 기록
- 메트릭 수집
- 기본적인 rate limit 적용
- upstream 호출에 대한 timeout / retry 제어
- tracing header 처리
- 간단한 라우팅 및 보호 정책 적용

즉, 비즈니스 로직은 본 서비스가 담당하고, 공통 운영 정책은 sidecar가 담당합니다.

## 핵심 기능

### JWT 검증

Sidecargo는 토큰을 발급하지 않고 검증만 수행합니다.

- Auth Service가 private key로 JWT를 발급
- Auth Service가 JWKS endpoint를 제공
- Sidecargo가 JWKS를 가져와 캐싱
- 토큰 헤더의 `kid`에 맞는 public key로 서명 검증

이 구조를 통해 발급 책임과 검증 책임을 분리하고, 키 회전도 sidecar 재배포 없이 처리할 수 있습니다.

### Request ID 생성 및 전파

요청에 request ID가 없으면 sidecar가 새로 생성하고, 이미 존재하면 그대로 전달합니다.

이 값은 로그, 메트릭, 추적 정보와 함께 사용되어 요청 흐름을 한 번에 따라갈 수 있게 합니다.

### Access Log

각 요청에 대해 다음과 같은 정보를 일관되게 기록하는 것을 목표로 합니다.

- request ID
- method / path
- response status
- latency
- caller 정보 또는 claim 기반 subject
- retry 여부

이를 통해 서비스 코드 안에 개별 로그를 흩뿌리지 않고도 기본 관측성을 확보할 수 있습니다.

### Metrics 수집

Sidecargo는 서비스 간 통신의 상태를 수치로 관찰할 수 있게 합니다.

예상 메트릭은 다음과 같습니다.

- 총 요청 수
- 상태 코드별 요청 수
- 응답 지연 시간
- 인증 실패 수
- rate limit 차단 수
- retry 발생 수

### Rate Limit

지정된 라우트 또는 호출 주체 기준으로 기본적인 rate limit을 적용합니다.

이를 통해 특정 클라이언트나 서비스의 과도한 호출이 downstream 서비스에 직접 부담을 주지 않도록 제어할 수 있습니다.

### Timeout / Retry

Sidecargo는 upstream 호출 시 기본 timeout을 적용하고, 정책에 따라 제한적인 retry를 수행합니다.

이 기능은 느리거나 불안정한 downstream 호출이 전체 서비스에 전파되는 것을 완화하는 데 목적이 있습니다.

### Tracing Header 처리

분산 추적 시스템과 연동할 수 있도록 tracing header를 보존하거나 생성하여 전달합니다.

이를 통해 sidecar를 경유한 요청도 전체 호출 체인 안에서 자연스럽게 연결됩니다.

### 간단한 라우팅 / 보호 로직

모든 요청을 동일하게 처리하지 않고, 라우트별로 최소한의 정책을 적용하는 것을 목표로 합니다.

예를 들면 다음과 같습니다.

- `/health` 같은 공개 엔드포인트는 인증 제외
- `/internal/*` 같은 경로는 JWT 필수
- 특정 라우트에 더 엄격한 timeout 또는 rate limit 적용

## 요청 처리 흐름

예상하는 기본 흐름은 다음과 같습니다.

1. 요청 수신
2. request ID 확인 또는 생성
3. tracing header 확인 및 전파 준비
4. 보호 대상 라우트라면 JWT 검증 수행
5. rate limit 검사
6. upstream으로 요청 전달
7. timeout / retry 정책 적용
8. access log와 metrics 기록

## 왜 Sidecar인가

이 프로젝트는 다음과 같은 문제를 줄이기 위해 sidecar 형태를 택합니다.

- 각 서비스가 인증과 운영 정책을 중복 구현하는 문제
- 서비스마다 로그 포맷과 메트릭 기준이 달라지는 문제
- 공통 네트워크 정책을 코드 레벨에서 일관되게 적용하기 어려운 문제

sidecar를 두면 서비스는 비즈니스 로직에 집중하고, 인증과 관측성, 호출 제어는 sidecar가 공통 계층으로 담당할 수 있습니다.

## 범위

Sidecargo는 다음 범위를 목표로 합니다.

- 경량 HTTP 기반 sidecar
- 서비스 간 내부 통신 보호
- 인증, 추적, 관측성, 호출 제어의 공통 처리

다음은 현재 목표 범위 밖입니다.

- 대규모 전사 API Gateway 대체
- 전체 클러스터 수준의 Service Mesh 제어 plane
- 복잡한 L7 정책 엔진 전체 구현

## 방향

초기 구현은 작은 단위부터 시작합니다.

1. JWKS 기반 공개키 조회 및 캐싱
2. JWT 검증 미들웨어
3. request ID / access log / metrics
4. rate limit
5. timeout / retry
6. 라우팅 및 보호 정책 확장
