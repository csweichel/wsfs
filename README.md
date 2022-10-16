## How to try
```bash
# make sure you have fusermount in the path
sudo apt-get install fuse

mkdir /tmp/remote

# prepare test content
git clone https://github.com/gitpod-io/gitpod /tmp/gitpod
cd /tmp/gitpod && tar cvvf /tmp/remote/gitpod.tar .; cd -

# preapre index
go run main.go -v index generate /tmp/gitpod-idx /tmp/gitpod.tar
cd /tmp/gitpod-idx && tar cvvfz /tmp/remote/gitpod.index .; cd -


# mount filesystem from local index OR
go run main.go mount local -v /tmp/gitpod-idx /tmp/remote/gitpod.tar /tmp/test123

# mount filesystem from remote index
(curl lama.sh | sh -s -- -d /tmp/remote) &
go run main.go mount remote -v http://localhost:8080/gitpod /tmp/test123
```