package api

//func SuggestTags(c *gin.Context) {
//	var json models.SuggestionTagsJSON
//	if err := c.ShouldBindJSON(&json); err != nil {
//		c.JSON(http.StatusBadRequest, models.R(models.KErrorInvalid, models.KReturnFalse, models.RDC{"tags": ""}))
//		return
//	}
//	session := json.Session
//	userid, err := cache.GetUserIDByUserSession(session)
//	if err != nil {
//		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnTrue, models.RDC{"userid": ""}))
//		return
//	}
//	if userid == "" {
//		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
//		c.Abort()
//		return
//	}
//	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"userid": userid}))
//	return
//}
