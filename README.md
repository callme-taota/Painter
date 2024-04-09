# PainterBlog-Server[简体中文](https://github.com/callme-taota/Painter-Blog/blob/Server/README_zh.md)

## Description
This is backend server of painter blog project. Which can help you to easily build a blog your self.  
Based on golang, you can easily start without golang environment if you have docker environment.

## What can you use this for
1. Build a blog for yourself.
2. Change font-end if you don't like without writing any back-end code. 

## Quick Start

To start this backend you have to prepare something. And make it accessible.
1. Mysql
2. Redis

### With go environment
```bash
git clone -b Server https://github.com/callme-taota/Painter-Blog.git
cd Painter-Blog
go mod download
go build -o painter-server-new .
./painter-server-new
```

### Docker
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

## Directory structure
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

## Config
You can change some config in `/conf/conf.json` before start.  
There is three part of config file.You can change some config of server. And also you can set redis and mysql connection config.  
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

### Note
If you are using docker to build server, you can set the environment instead of using config file. The priority of environment variables is higher than that of configuration files.

## APIs
[Full APIs](https://github.com/callme-taota/Painter-Blog/blob/Server/API.md)

The table below not contain every api. You can find full api at [here](https://github.com/callme-taota/Painter-Blog/blob/Server/API.md).

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



## ToDo
1. Find some bug?
2. Add plug-in support.
3. Add comment.
4. Add more cache support.
5. Add test.
6. Add suggest system.
7. Add file upload download support.
8. Add search support.


## Build project better
Create some issues let me know, or join me the complete code.  
You can find me at my [website](http://callmetaota.fun).