export const validateEmail = (email: string) : boolean =>  {
    // 邮箱验证正则
    var reg = /^[A-Za-z0-9]+([_\.][A-Za-z0-9]+)*@([A-Za-z0-9\-]+\.)+[A-Za-z]{2,6}$/;
    return reg.test(email);
}

export const validateUserID = (id: string): boolean => {
    var reg = /^(?![0-9])[a-zA-Z0-9,./<>?*&%$!]*$/
    return reg.test(id)
}

export const validatePassword = (password: string): boolean => {
    var reg = /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z\d]{8,}$/
    return reg.test(password)
}