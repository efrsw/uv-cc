BASEPORT ?= 47073
NODES ?= 5
DATADIR ?= ./tmp
DATAFILE ?= ../data/reviews_100m.csv

build:
	go build -o ./bin/client ./client/
	go build -o ./bin/bucket ./bucket/

clean:
	rm -rf $(DATADIR)/*
	killall bucket 2>/dev/null || true

start-buckets:
	@for i in 0 1 2 3 4; do \
		./bin/bucket -idx $$i -baseport $(BASEPORT) -nodes $(NODES) -datadir $(DATADIR) & \
	done
	@sleep 2
	@echo "Buckets started"

stop-buckets:
	@killall bucket 2>/dev/null || true
	@echo "Buckets stopped"

report: build stop-buckets clean start-buckets
	./bin/client upload -baseport $(BASEPORT) -nodes $(NODES) -file $(DATAFILE) -max 1
	./bin/client search -baseport $(BASEPORT) -nodes $(NODES) -key 277693
	./bin/client delete -baseport $(BASEPORT) -nodes $(NODES) -key 277693
	./bin/client merge -dir $(DATADIR) -output ./report.log
	$(MAKE) stop-buckets

.PHONY: build clean start-buckets stop-buckets report
