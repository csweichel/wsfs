#!/bin/bash

prepare_demo_content() {
    echo -e "\n\e[1;32mPreparing demo content ...\e[0m\n"

    mkdir -p /workspace/wsfs-demo
    mkdir -p /workspace/wsfs-demo/remote
    pushd /workspace/wsfs-demo

    git clone https://github.com/gitpod-io/gitpod /tmp/gitpod
    cd /tmp/gitpod
    tar cf /workspace/wsfs-demo/remote/gitpod.tar .
    cd ..

    wsfs index generate index /workspace/wsfs-demo/remote/gitpod.tar
    cd index
    tar cfz /workspace/wsfs-demo/remote/gitpod.index .
    cd ..
    rm -rf index
    popd

    echo -e "\n\e[1;32mPrepared demo content in /workspace/wsfs-demo/remote:\e[0m"
    ls -lhn /workspace/wsfs-demo/remote
}

install_wsfs() {
    go install
}

serve_index() {
    curl lama.sh | sh -s -- -d /workspace/wsfs-demo/remote
}

prepare_mount() {
    sudo apt-get install -y fuse-overlayfs    
}

mount_wsfs() {
    mkdir -p /workspace/wsfs-demo/lower
    wsfs mount remote --daemonise http://localhost:8080/gitpod /workspace/wsfs-demo/lower
}

mount_overlay() {
    mkdir -p /workspace/wsfs-demo/upper
    mkdir -p /workspace/wsfs-demo/work
    mkdir -p /workspace/wsfs-demo/mnt

    fuse-overlayfs -o lowerdir=/workspace/wsfs-demo/lower,upperdir=/workspace/wsfs-demo/upper,workdir=/workspace/wsfs-demo/work /workspace/wsfs-demo/mnt
}

print_demo_info() {
    echo -e "\n\e[1;1mFilesystem mounted in \e[1;33m$(cat /tmp/wsfs.log  | grep mounted | cut -d ' ' -f 3)\e[0m\n"
    
    echo -e "$(cat<<EOF
\e[1;1mWhat just happend?\e[0m
We just mounted the entire Gitpod repo in less than 100ms, using a remote index and a tar file.
On top of this filesystem, we mounted (fuse-)overlayfs which enables modification of the original content.

\e[1;1mWhere does the index come from?\e[0m
The index is served using lama.sh from /workspace/wsfs-demo/remote/gitpod.index
You can inspect the index using:
    wsfs index dump http://localhost:8080/gitpod > /tmp/gitpod-index.json

\e[1;1mWhere does contect come from?\e[0m
The index is served using lama.sh from /workspace/wsfs-demo/remote/gitpod.tar

\e[1;1mSo what's mounted now?\e[0m
$(mount | grep wsfs-demo)

\e[1;1mCan we use this in production?\e[0m
Nope - not yet. This is just a PoC and there's heaps of development, testing and benchmarking to do.
\n

EOF
)"
}
