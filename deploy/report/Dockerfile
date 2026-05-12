FROM izakmarais/grafana-reporter:latest

# (옵션) 커스텀 LaTeX 템플릿이 있다면 컨테이너 내부로 복사합니다.
# COPY ./templates /app/templates

# 실행 시 기본 인자 설정을 위한 환경 변수 (Compose에서 덮어쓰기 가능)
ENV GF_PROTO="http://"
ENV GF_HOST="grafana:3000"
ENV GF_API_KEY="your_api_key_here"

# 컨테이너 시작 시 실행될 명령어
ENTRYPOINT [ "/usr/local/bin/grafana-reporter" ]
CMD [ "-proto", "${GF_PROTO}", "-ip", "${GF_HOST}", "-api-key", "${GF_API_KEY}" ]