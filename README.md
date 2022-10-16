## How to try
```bash
# make sure you have fusermount in the path
sudo apt-get install fuse

# prepare test content
git clone https://github.com/gitpod-io/gitpod /tmp/gitpod
cd /tmp/gitpod && tar cvvf ../gitpod.tar .; cd -

# preapre index
go run main.go -v index generate /tmp/gitpod-idx /tmp/gitpod.tar

# mount filesystem
go run main.go mount -v /tmp/gitpod-idx /tmp/gitpod.tar /tmp/test123
```