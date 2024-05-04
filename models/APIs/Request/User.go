package Request

type UserSignUpJSON struct {
	UserName    string `json:"username"`
	NickName    string `json:"nickname"`
	Email       string `json:"email"`
	PhoneNum    int    `json:"phonenum"`
	HeaderField string `json:"headerfield"`
	Password    string `json:"password"`
}

type CheckUsingSessionJSON struct {
	Session string `json:"session"`
}

type LoginUsingEmailJson struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUsingPhoneJson struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type LoginUsingUserNameJson struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type LogOutJson struct {
	Session string `json:"session"`
}

type UserNameUpdateJson struct {
	Name string `json:"name"`
}

type UserEmailUpdateJson struct {
	Email string `json:"email"`
}

type UserNickNameUpdateJson struct {
	NickName string `json:"nickname"`
}

type UserPhoneUpdateJson struct {
	PhoneNum int `json:"phonenum"`
}

type UserHeaderFieldJson struct {
	HeaderField string `json:"headerfield"`
}

type UserProfileJson struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	NickName string `json:"nickname"`
	PhoneNum int    `json:"phonenum"`
}

type UserResetPasswordJSON struct {
	ID          int    `json:"ID"`
	OldPassword string `json:"OldPassword"`
	NewPassword string `json:"NewPassword"`
}

type UserIDJSON struct {
	ID int `json:"ID"`
}
