# API Documentation

# Still working!!!!

## Overview
This document outlines the design and usage guidelines for the API, including the list of endpoints, request and response formats, error handling, etc.   
[Project information](https://github.com/callme-taota/Painter-Blog/blob/Server/README.md)

## Basic 
- Data format : JSON
- Authorization : Session based

## Return format
```
{
  "msg": Return msg list,
  "ok": Boolean,
  "data": Object(Main part of result)
}
```

### Return message list  
```
ReturnMsgOK      = "ok"
ReturnMsgSuccess = "success"
ReturnMsgError   = "error"

ErrorInvalid          = "post data invalid"
ErrorMissing          = "post data missing"
ErrorTypeError        = "post data type error"
ErrorPermissionDenied = "permission denied"
ErrorNotFound         = "not found"
ErrorNoUser           = "user not found"
ErrorSessionInvalid   = "user session invalid"
ErrorPassword         = "password error"
```

## Endpoint List

### USER

#### User sign up
    - URL: '/user/create'
    - Method: POST
    - LoginRequired: No
##### Request Parameters
`"username"` string  
`"nickname"` string  
`"email"` string  
`"phonenum"` int  
`"headerfield"` string  
`"password"` string  
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.userid"` int

##### Request Sample
```json
{
  "username": "example",
  "nickname": "John",
  "email": "john@example.com",
  "phonenum": 1234567890,
  "headerfield": "some value",
  "password": "password123"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "userid": 12345
  }
}
```

#### User login (email)
    - URL: '/user/login/email'
    - Method: POST
    - LoginRequired: No
##### Request Parameters
`"email"` string  
`"password"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.Session"` string

`Header` `Set-session` string

##### Request Sample
```json
{
  "email": "john@example.com",
  "password": "password123"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "Session": "some random session"
  }
}
```

#### User login (phone)
    - URL: '/user/login/phone'
    - Method: POST
    - LoginRequired: No
##### Request Parameters
`"phone"` string  
`"password"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.Session"` string

`Header` `Set-session` string

##### Request Sample
```json
{
  "phone": "123 0000 0000",
  "password": "password123"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "Session": "some random session"
  }
}
```

#### User login (user name)
    - URL: '/user/login/uname'
    - Method: POST
    - LoginRequired: No
##### Request Parameters
`"username"` string  
`"password"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.Session"` string

`Header` `Set-session` string

##### Request Sample
```json
{
  "username": "123",
  "password": "password123"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "Session": "some random session"
  }
}
```

#### User login check
    - URL: '/user/login/check'
    - Method: POST
    - LoginRequired: No

WARN: Direct post session is not safe in some case. Don't let session accessible in font-end js.

##### Request Parameters
`"session"` string  
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.userid"` int

##### Request Sample
```json
{
  "session": "some random session"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "userid": 1234
  }
}
```

#### User self information
    - URL: '/user/self'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
Nothing
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.id"` int  
`"data.username"` string  
`"data.nickname"` string  
`"data.email"` string  
`"data.adminflag"` bool  
`"data.lastlogin"` timestamp(second)  
`"data.phonenum"` int  
`"data.headerfield"` string  

##### Request Sample
```json
{
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "id": 12345,
    "username": "example",
    "nickname": "John",
    "email": "john@example.com",
    "adminflag": true,
    "lastlogin": 1647312000,
    "phonenum": 1234567890,
    "headerfield": "some value"
  }
}
```

#### User full self information
    - URL: '/user/self/full'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
Nothing
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   
`"data.articlenumber"` int  
`"data.articlelist"` list  
`"data.collectionnumber"` int    
`"data.followingnumber"` int    
`"data.followernumber"` int    
`"data.userinfo"` object  
`"data.userinfo.username"` string  
`"data.userinfo.nickname"` string  
`"data.userinfo.email"` string  
`"data.userinfo.adminflag"` bool  
`"data.userinfo.lastlogin"` timestamp(second)  
`"data.userinfo.phonenum"` int  
`"data.userinfo.headerfield"` string

##### Request Sample
```json
{
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "articlenumber": 10,
    "articlelist": [...],
    "collectionnumber": 5,
    "followingnumber": 20,
    "followernumber": 15,
    "userinfo": {
      "username": "example",
      "nickname": "John",
      "email": "john@example.com",
      "adminflag": true,
      "lastlogin": 1647312000,
      "phonenum": 1234567890,
      "headerfield": "some value"
    }
  }
}
```

#### User logout
    - URL: '/user/logout'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
Nothing
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object   

`Header` `Set-session` ""

##### Request Sample
```json
{
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
  }
}
```

#### User update name
    - URL: '/user/update/name'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"name"` string  
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.name"` string  

##### Request Sample
```json
{
  "name": ""
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "name": "name"
  }
}
```

#### User update email
    - URL: '/user/update/email'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"email"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.email"` string

##### Request Sample
```json
{
  "email": ""
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "email": "123456@example.com"
  }
}
```

#### User update nickname
    - URL: '/user/update/nickname'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"nickname"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.nickname"` string

##### Request Sample
```json
{
  "nickname": "nickname123"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "nickname": "nickname123"
  }
}
```

#### User update phone
    - URL: '/user/update/phone'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"phonenum"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.phone"` string

##### Request Sample
```json
{
  "phonenum": "123 0000 0000"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "phone": "123 0000 0000"
  }
}
```

#### User update header field
    - URL: '/user/update/headerfield'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"headerfield"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.headerfield"` string

##### Request Sample
```json
{
  "headerfield": "url address"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "headerfield": "url address"
  }
}
```

#### User update user profile
    - URL: '/user/update'
    - Method: POST
    - LoginRequired: Yes

##### Request Parameters
`"headerfield"` string  
`"email"` string  
`"nickname"` string  
`"headerfield` string  
`"phonenum"` string
##### Response Parameters
`"msg"` string  
`"ok"` bool  
`"data"` object  
`"data.email"` string  
`"data.name"` string  
`"data.nickname"` string
`"data.headerfield"` string
`"data.phonenum"` string

##### Request Sample
```json
{
  "email": "example@example.com",
  "name": "John Doe",
  "nickname": "Johnny",
  "headerfield": "url address",
  "phonenum": "1234567890"
}
```
##### Response Sample
```json
{
  "msg": "success",
  "ok": true,
  "data": {
    "email": "example@example.com",
    "name": "John Doe",
    "nickname": "Johnny",
    "headerfield": "url address",
    "phonenum": "1234567890"
  }
}
```