all:	release

release:
	go build -ldflags='-s' -o docker-ovs-plugin .

docker-ovs-plugin.spec: docker-ovs-plugin.spec.in
	$$( \
		GITREF=$$(git show-ref --hash HEAD); \
		VERSION=$$(git describe --dirty 2>/dev/null|| echo -n "0.0.0git$$GITREF"); \
		m4 -DVERSION=$$VERSION -DGITREF=$$GITREF $< > $@; \
	)


docker-ovs-plugin.service: docker-ovs-plugin.service.in
	m4 -DBINDIR=/usr/bin $< > $@

rpm:	docker-ovs-plugin.spec docker-ovs-plugin.service
	bash buildrpm.sh

clean:
	rm -f docker-ovs-plugin.spec docker-ovs-plugin.service
