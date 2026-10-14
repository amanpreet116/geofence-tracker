# Root convenience targets. Override variables per call, e.g.
#   make simulate VEHICLE=CA-TRK-1001,CA-VAN-2002 ROUTE=bridge SPEED=150
COMPOSE  ?= docker compose
VEHICLE  ?= CA-TRK-1001
ROUTE    ?= restricted
SPEED    ?= 250
INTERVAL ?= 1s

# Docker Hub publishing (make docker-push DOCKERHUB_USER=yourname)
DOCKERHUB_USER ?=
IMAGE_NAME     ?= geofence-tracker-backend
VERSION        ?= v1.0.0
PLATFORMS      ?= linux/amd64,linux/arm64
IMAGE           = $(DOCKERHUB_USER)/$(IMAGE_NAME)
# Remote targets (make verify BASE_URL=https://api.example.com)
BASE_URL       ?= http://localhost:8080

.PHONY: up down clean logs ps seed simulate test test-db docker-builder docker-build docker-push verify seed-remote

up:            ## build and start db, backend and frontend (detached)
	$(COMPOSE) up -d --build --wait

down:          ## stop containers (keeps data)
	$(COMPOSE) down

clean:         ## stop containers and DELETE the database volume
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=100 backend

ps:
	$(COMPOSE) ps

seed:          ## load demo data through the API (idempotent)
	$(COMPOSE) exec backend /app/seed -base-url http://localhost:8080

simulate:      ## drive vehicles through the restricted zone (Ctrl-C to stop)
	$(COMPOSE) exec backend /app/simulator -base-url http://localhost:8080 \
		-vehicle $(VEHICLE) -route $(ROUTE) -speed $(SPEED) -interval $(INTERVAL)

test:          ## backend build + vet + unit tests
	$(MAKE) -C backend check

test-db:       ## backend tests including PostGIS integration tests (needs `make up`)
	$(MAKE) -C backend test-db DB_HOST_PORT=$${DB_HOST_PORT:-5432}

docker-builder: ## one-time: a buildx builder that can produce multi-arch images
	docker buildx inspect geofence-builder >/dev/null 2>&1 || docker buildx create --name geofence-builder --driver docker-container
	docker buildx use geofence-builder

docker-build:  ## build the multi-arch backend image locally (no push) to check it builds
	docker buildx build --builder geofence-builder --platform $(PLATFORMS) ./backend

docker-push: docker-builder  ## build + push :latest and :$(VERSION) for $(PLATFORMS)
	@test -n "$(DOCKERHUB_USER)" || (echo "set DOCKERHUB_USER, e.g. make docker-push DOCKERHUB_USER=jane"; exit 1)
	docker buildx build --builder geofence-builder --platform $(PLATFORMS) \
		-t $(IMAGE):$(VERSION) -t $(IMAGE):latest --push ./backend

verify:        ## API contract check against BASE_URL
	BASE_URL=$(BASE_URL) scripts/verify_contract.sh

seed-remote:   ## seed demo data into BASE_URL using the published image (no Go needed)
	@test -n "$(DOCKERHUB_USER)" || (echo "set DOCKERHUB_USER"; exit 1)
	docker run --rm --entrypoint /app/seed $(IMAGE):$(VERSION) -base-url $(BASE_URL)
