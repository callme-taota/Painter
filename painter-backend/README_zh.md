# PainterBlog-server[English](https://github.com/callme-taota/Painter-Blog/blob/Server/README.md)

## 介绍
这是PainterBlog的后端服务端。这可以帮助你轻松地建立自己的博客。
基于golang，如果你有docker环境，你可以在没有golang环境的情况下轻松启动。

## 你可以基于这个项目干嘛
1. 创建一个属于自己的博客
2. 使用这个后端提供的接口自行实现一个前端如果你不喜欢默认的前端

## 快速启动

开启这个项目你需要准备一些东西，并让其可连接。
1. Mysql
2. Redis

### 有Go环境
```bash
git clone -b Server https://github.com/callme-taota/Painter-Blog.git
cd Painter-Blog
go mod download
go build -o painter-server-new .
./painter-server-new
```

### Docker环境
```bash
git clone -b Server https://github.com/callme-taota/Painter-Blog.git
cd Painter-Blog
docker build -t painter-server .
docker run -p 3003:3003 painter-server
```

| Environment Option | Description                                                                 |
|--------------------|-----------------------------------------------------------------------------|
| SERVER_PORT        | The port on which the server should listen for incoming connections.        |
| SERVER_MODEL       | The model used by the server: debug, release, test.                         |
| REDIS_HOST         | The host where the Redis server is running.                                 |
| REDIS_PORT         | The port on which the Redis server is listening for connections.            |
| REDIS_PASSWORD     | The password required to authenticate with the Redis server, if applicable. |
| REDIS_DB           | The Redis database number to use.                                           |
| MYSQL_USER         | The username used to authenticate with the MySQL server.                    |
| MYSQL_PASS         | The password used to authenticate with the MySQL server.                    |
| MYSQL_HOST         | The host where the MySQL server is running.                                 |
| MYSQL_PORT         | The port on which the MySQL server is listening for connections.            |
| MYSQL_DB           | The name of the MySQL database to use.                                      |

## 项目结构
```
├── Dockerfile                         # Dockerfile for containerization
├── README.md                          # Main README file in English
├── README_zh.md                       # README file in Chinese
├── api                                # API related files
│   ├── Article.go                     # API for articles
│   ├── Category.go                    # API for categories
│   ├── Collection.go                  # API for collections
│   ├── Comment.go                     # API for comments
│   ├── Follow.go                      # API for following
│   ├── History.go                     # API for history
│   ├── Tag.go                         # API for tags
│   └── User.go                        # API for users
├── cache                              # Cache related files
│   ├── Cache.go                       # Cache interface
│   ├── PortCache.go                   # Not complete yet.
│   └── User.go                        # Cache implementation for users
├── conf                               # Configuration files
│   ├── Conf.go                        # Configuration interface
│   └── conf.json                      # Sample configuration file
├── database                           # Database related files
│   ├── Article.go                     # Database model and function for articles
│   ├── Category.go                    # Database model and function for categories
│   ├── Collection.go                  # Database model and function for collections
│   ├── Comment.go                     # Database model and function for comments
│   ├── DB.go                          # Database interface
│   ├── Follow.go                      # Database model and function for follow relationships
│   ├── History.go                     # Database model and function for history
│   ├── Migrate.go                     # Database migration
│   ├── Tag.go                         # Database model and function for tags
│   └── User.go                        # Database model and function for users
├── go.mod                             # Go module file
├── go.sum                             # Go dependencies checksum file
├── main.go                            # Main entry point of the application
├── models                             # Model related files
│   ├── APIs                           # API models include request and response
│   │   ├── Article.go                 # Model for articles
│   │   ├── Category.go                # Model for categories
│   │   ├── Collection.go              # Model for collections
│   │   ├── Comment.go                 # Model for comments
│   │   ├── Follow.go                  # Model for following
│   │   ├── History.go                 # Model for history
│   │   ├── Tags.go                    # Model for tags
│   │   └── User.go                    # Model for users
│   ├── Cache.go                       # Cache model
│   ├── Models.go                      # Generic models
│   └── Mysql.go                       # MySQL specific models
├── server                             # Server related files
│   ├── Cros.go                        # CORS related functionality
│   ├── GinMode.go                     # Gin server mode configuration
│   ├── Link.go                        # Link related functionality
│   └── Server.go                      # Server initialization
├── tolog                              # Logging related files
│   ├── logs                           # Folder for log files
│   └── tolog.go                       # Logging interface
└── utils                              # Utility files
    ├── CheckJsonMissing.go            # Utility for checking missing JSON keys
    ├── CreateUserSession.go           # Utility for creating user sessions
    ├── EncryptionPassword.go          # Utility for password encryption
    └── JSONReader.go                  # Utility for reading JSON data
```

## 配置文件
你可以在`/conf/conf.json`更改一些设置在运行之前。
```json
{
    "server" : {
      "name" : "GoServer-QuickStart",
      "version" : "1.0.1",
      "port" : "3003",
      "model" : "debug",
      "author" : "Taota"
    },
    "redis" : {
        "host": "localhost",
        "port":"6379",
        "DB": "0",
        "password": ""
    },
    "mysql": {
      "user": "root",
      "password": "Mysqlroot",
      "port": "3306",
      "host": "localhost",
      "database": "painter-blog-new"
    }
}
```

### 提示
如果你使用docker构建，那么环境变量的优先级高于配置文件。

## APIs
[完整 API](https://github.com/callme-taota/Painter-Blog/blob/Server/API_zh.md)

下表没有将全部API列出，你可以在这找到[全部API](https://github.com/callme-taota/Painter-Blog/blob/Server/API_zh.md)

| Endpoint                    | Method | Description                                     |
|-----------------------------|--------|-------------------------------------------------|
| `/user/create`              | POST   | Creates a new user account                      |
| `/user/login`               | POST   | Logs in a user                                  |
| `/user/self`                | GET    | Get user it self's data                         |
| `/tag/suggest`              | GET    | Retrieves suggested tags                        |
| `/tag/list`                 | GET    | Retrieves list of tags                          |
| `/history/list`             | GET    | Retrieves user's history                        |
| `/follow/followers`         | GET    | Retrieves user's followers                      |
| `/follow/followings`        | GET    | Retrieves user's followings                     |
| `/comment/list`             | GET    | Retrieves comments by article                   |
| `/comment/list/l`           | GET    | Retrieves comments by article with liked status |
| `/collection/list`          | POST   | Retrieves user's collections                    |
| `/collection/check`         | POST   | Checks if an article is in a collection         |
| `/category/list`            | GET    | Retrieves list of categories                    |
| `/category/get`             | GET    | Retrieves category                              |
| `/article/get/author`       | GET    | Retrieves articles by author                    |
| `/article/get/title`        | GET    | Retrieves articles by title                     |
| `/article/get/content`      | GET    | Retrieves articles by content                   |
| `/article/get/collection`   | GET    | Retrieves articles by collection                |
| `/article/get/category`     | GET    | Retrieves articles by category                  |
| `/article/get/tag`          | GET    | Retrieves articles by tag                       |
| `/article/get`              | GET    | Retrieves full article                          |
| `/article/like`             | POST   | Likes an article                                |
| `/article/like/check`       | POST   | Checks if a user liked an article               |
| `/article/collection`       | POST   | Adds an article to collection                   |
| `/article/tag/create`       | POST   | Creates a tag for an article                    |
| `/article/tag/delete`       | POST   | Deletes a tag from an article                   |

## 让项目更好
建Issues或者直接加入我一起干。   
你可以在我的[网站](http://callmetaota.fun)找到我。