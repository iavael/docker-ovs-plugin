docker-ovs-plugin
=================

### QuickStart Instructions

The quickstart instructions describe how to start the plugin in **nat mode**. Flat mode is described in the `flat` mode section.

**1.** Make sure you are using Docker 1.9 or later

**2.** You need to `modprobe openvswitch` on the machine where the Docker Daemon is located

**3.** Start docker-ovs-plugin container

```sh
$ docker run -d --net=host --privileged iavael/docker-ovs-plugin
```

**4.** Now you are ready to create a new network

```sh
$ docker network create -d ovs openvswitch
```

**6.** Test it out!

```
$ docker run -itd --net=openvswitch --name=web nginx

$ docker run -it --rm --net=openvswitch busybox wget -qO- http://web
```

### Additional Notes:

 - To view the Open vSwitch configuration, use `ovs-vsctl show`.
 - To view the OVSDB tables, run `ovsdb-client dump`. All of the mentioned OVS utils are part of the standard binary installations with very well documented [man pages](http://openvswitch.org/support/dist-docs/).
 - The containers are brought up on a flat bridge. This means there is no NATing occurring. A layer 2 adjacency such as a VLAN or overlay tunnel is required for multi-host communications.

### Hacking and Contributing

Yes!! Please see issues for todos or add todos into [issues](https://github.com/iavael/docker-ovs-plugin/issues)! Only rule here is no jerks.

1. Install [Go](https://golang.org/doc/install). OVS as listed above and a kernel >= 3.19.

2. Clone and start the OVS plugin:

    ```
    mkdir -p $GOPATH/src/github.com/iavael
    cd $GOPATH/src/github.com/iavael
    git clone https://github.com/iavael/docker-ovs-plugin.git
    cd docker-ovs-plugin
    go run main.go -debug
    # or using explicit configuration flags:
    go run main.go -debug -host=172.18.40.1 -port=6640
    ```

3. The rest is the same as the Quickstart Section.

 **Note:** If you are new to Go.

 - Go compile times are very fast due to linking being done statically. In order to link the libraries, Go looks for source code in the `$GOPATH/src/` directory.
 - Typically you would clone the project to a directory like so `$GOPATH/src/github.com/iavael/docker-ovs-plugin/`. Go knows where to look for the root of the go code, binaries and pkgs based on the `$GOPATH` shell ENV.
 - For example, you would clone to the path `/home/<username>/go/src/github.com/iavael/docker-ovs-plugin/` and put `export GOPATH=/home/<username>/go` in wherever you store your persistent ENVs in places like `~/.bashrc`, `~/.profile` or `~/.bash_profile` depending on the OS and system configuration.


### Thanks

Thanks to the guys at [Weave](http://weave.works) for writing their awesome [plugin](https://github.com/weaveworks/docker-plugin). We borrowed a lot of code from here to make this happen!
