start_otel:
	@docker

# e2e 测试

e2e:
	sh ./script/integrate_test.sh

e2e_up:
	docker compose -f script/cache_test_compose.yml up -d

e2e_down:
	docker compose -f script/cache_test_compose.yml down