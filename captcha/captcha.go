package captcha

import (
	"github.com/google/uuid"
	"github.com/mojocn/base64Captcha"
)

// SetStore 设置store
func SetStore(s base64Captcha.Store) {
	base64Captcha.DefaultMemStore = s
}

// configJsonBody json request body.
type configJsonBody struct {
	Id            string
	CaptchaType   string
	VerifyValue   string
	DriverAudio   *base64Captcha.DriverAudio
	DriverString  *base64Captcha.DriverString
	DriverChinese *base64Captcha.DriverChinese
	DriverMath    *base64Captcha.DriverMath
	DriverDigit   *base64Captcha.DriverDigit
}

func DriverDigitFunc() (id, b64s, answer string, err error) {
	e := configJsonBody{}
	e.Id = uuid.New().String()
	e.DriverDigit = base64Captcha.NewDriverDigit(80, 240, 6, 0.7, 80)
	driver := e.DriverDigit
	captcha := base64Captcha.NewCaptcha(driver, base64Captcha.DefaultMemStore)
	return captcha.Generate()
}

// Verify 校验验证码
func Verify(id, code string, clear bool) bool {
	return base64Captcha.DefaultMemStore.Verify(id, code, clear)
}
