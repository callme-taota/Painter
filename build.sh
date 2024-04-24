cd ./server/static/webroot

if ! command -v node &> /dev/null; then
    echo "Node.js not found, please install Node.js."
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Go not found, please install Golang."
    exit 1
fi

node_version=$(node -v)
echo "Node.js Version：$node_version"
node_version=$(node -v | cut -c 2- | cut -d . -f 1)
if [ "$node_version" -lt 20 ]; then
    echo "Node.js version is less than 20, check your node.js version. "
    exit 1
fi

go_version=$(go version | awk '{print $3}' | cut -c3-)
echo "Go Version：$go_version"
go_version=$(go version | awk '{print $3}' | cut -c3- | cut -d '.' -f 2)
if [ "$go_version" -lt 20 ]; then
    echo "Go version is less than 1.20, check your Golang version. "
    exit 1
fi

echo "Build vue"
npm install
npm run build

cd ..
cd ..
cd ..

echo "Build painter"
export CGO_ENABLED=0
go build -o painter
echo "Build success"
