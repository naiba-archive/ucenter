package engine

import (
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/naiba/com"

	"github.com/naiba/ucenter/pkg/ram"

	"github.com/RangelReale/osin"
	"github.com/felipeweb/osin-mysql"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/naiba/ucenter"
	"github.com/naiba/ucenter/pkg/nbgin"

	// MySQL Driver
	_ "github.com/jinzhu/gorm/dialects/mysql"

	"gopkg.in/square/go-jose.v2"
)

var (
	osinStore        *mysql.Storage
	osinServer       *osin.Server
	openIDPublicKeys *jose.JSONWebKeySet
	jwtSigner        jose.Signer
)

func initOsinResource() {
	db, err := sql.Open("mysql", ucenter.DBDSN)
	if err != nil {
		panic(err)
	}

	osinStore = mysql.New(db, "osin_")
	err = osinStore.CreateSchemas()
	if err != nil {
		panic(err)
	}
	sconfig := osin.NewServerConfig()
	sconfig.AllowedAuthorizeTypes = osin.AllowedAuthorizeType{osin.CODE, osin.TOKEN}
	sconfig.AllowedAccessTypes = osin.AllowedAccessType{osin.AUTHORIZATION_CODE,
		osin.REFRESH_TOKEN, osin.PASSWORD, osin.CLIENT_CREDENTIALS, osin.ASSERTION}
	sconfig.AllowGetAccessRequest = true
	sconfig.AllowClientSecretInParams = true
	osinServer = osin.NewServer(sconfig, osinStore)

	// Load signing key.
	privateKeyBytes := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA4f5wg5l2hKsTeNem/V41fGnJm6gOdrj8ym3rFkEU/wT8RDtn
SgFEZOQpHEgQ7JL38xUfU0Y3g6aYw9QT0hJ7mCpz9Er5qLaMXJwZxzHzAahlfA0i
cqabvJOMvQtzD6uQv6wPEyZtDTWiQi9AXwBpHssPnpYGIn20ZZuNlX2BrClciHhC
PUIIZOQn/MmqTD31jSyjoQoV7MhhMTATKJx2XrHhR+1DcKJzQBSTAGnpYVaqpsAR
ap+nwRipr3nUTuxyGohBTSmjJ2usSeQXHI3bODIRe1AuTyHceAbewn8b462yEWKA
Rdpd9AjQW5SIVPfdsz5B6GlYQ5LdYKtznTuy7wIDAQABAoIBAQCwia1k7+2oZ2d3
n6agCAbqIE1QXfCmh41ZqJHbOY3oRQG3X1wpcGH4Gk+O+zDVTV2JszdcOt7E5dAy
MaomETAhRxB7hlIOnEN7WKm+dGNrKRvV0wDU5ReFMRHg31/Lnu8c+5BvGjZX+ky9
POIhFFYJqwCRlopGSUIxmVj5rSgtzk3iWOQXr+ah1bjEXvlxDOWkHN6YfpV5ThdE
KdBIPGEVqa63r9n2h+qazKrtiRqJqGnOrHzOECYbRFYhexsNFz7YT02xdfSHn7gM
IvabDDP/Qp0PjE1jdouiMaFHYnLBbgvlnZW9yuVf/rpXTUq/njxIXMmvmEyyvSDn
FcFikB8pAoGBAPF77hK4m3/rdGT7X8a/gwvZ2R121aBcdPwEaUhvj/36dx596zvY
mEOjrWfZhF083/nYWE2kVquj2wjs+otCLfifEEgXcVPTnEOPO9Zg3uNSL0nNQghj
FuD3iGLTUBCtM66oTe0jLSslHe8gLGEQqyMzHOzYxNqibxcOZIe8Qt0NAoGBAO+U
I5+XWjWEgDmvyC3TrOSf/KCGjtu0TSv30ipv27bDLMrpvPmD/5lpptTFwcxvVhCs
2b+chCjlghFSWFbBULBrfci2FtliClOVMYrlNBdUSJhf3aYSG2Doe6Bgt1n2CpNn
/iu37Y3NfemZBJA7hNl4dYe+f+uzM87cdQ214+jrAoGAXA0XxX8ll2+ToOLJsaNT
OvNB9h9Uc5qK5X5w+7G7O998BN2PC/MWp8H+2fVqpXgNENpNXttkRm1hk1dych86
EunfdPuqsX+as44oCyJGFHVBnWpm33eWQw9YqANRI+pCJzP08I5WK3osnPiwshd+
hR54yjgfYhBFNI7B95PmEQkCgYBzFSz7h1+s34Ycr8SvxsOBWxymG5zaCsUbPsL0
4aCgLScCHb9J+E86aVbbVFdglYa5Id7DPTL61ixhl7WZjujspeXZGSbmq0Kcnckb
mDgqkLECiOJW2NHP/j0McAkDLL4tysF8TLDO8gvuvzNC+WQ6drO2ThrypLVZQ+ry
eBIPmwKBgEZxhqa0gVvHQG/7Od69KWj4eJP28kq13RhKay8JOoN0vPmspXJo1HY3
CKuHRG+AP579dncdUnOMvfXOtkdM4vk0+hWASBQzM9xzVcztCa+koAugjVaLS9A+
9uQoqEeVNTckxx0S2bYevRy7hGQmUJTyQm3j1zEUR5jpdbL83Fbq
-----END RSA PRIVATE KEY-----`)
	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		log.Fatalf("no private key found")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse key: %v", err)
	}
	// Configure jwtSigner and public keys.
	privateKey := &jose.JSONWebKey{
		Key:       key,
		Algorithm: "RS256",
		Use:       "sig",
		KeyID:     "1", // KeyID should use the key thumbprint.
	}

	jwtSigner, err = jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, nil)
	if err != nil {
		log.Fatalf("failed to create jwtSigner: %v", err)
	}

	openIDPublicKeys = &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			jose.JSONWebKey{
				Key:       &key.PublicKey,
				Algorithm: "RS256",
				Use:       "sig",
				KeyID:     "1",
			},
		},
	}
}

// ServWeb 开启Web服务
func ServWeb() {
	initOsinResource()
	binding.Validator = new(nbgin.DefaultValidator)
	r := gin.Default()
	r.Static("static", "static")
	r.Static("upload", "upload")
	r.SetFuncMap(template.FuncMap{
		"df_allow": func(user *ucenter.User, perm string) bool {
			return ucenter.RAM.Enforce(user.StrID(), ram.DefaultDomain, ram.DefaultProject, perm)
		},
		"allow": func(user *ucenter.User, domain, project, perm string) bool {
			return ucenter.RAM.Enforce(user.StrID(), domain, project, perm)
		},
		"add": func(a, b int) int {
			return a + b
		},
	})
	r.LoadHTMLGlob("template/**/*")

	// 头像 Header
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/upload/avatar/") {
			c.Header("Content-Type", "image")
		}
	})

	// 鉴权
	r.Use(authorizeMiddleware)

	// 登录
	r.GET("/login", login)
	r.POST("/login", loginHandler)

	// 注册
	r.GET("/signup", signup)
	r.POST("/signup", signupHandler)

	// 用户中心
	mustLoginRoute := r.Group("")
	mustLoginRoute.Use(anonymousMustLogin)
	{
		mustLoginRoute.GET("/", index)
		mustLoginRoute.GET("/logout", logout)
		mustLoginRoute.PATCH("/", editProfileHandler)
		mustLoginRoute.DELETE("/:id", userDelete)
		mustLoginRoute.POST("/app", editOauth2App)
	}

	// 管理员路由
	admin := mustLoginRoute.Group("/admin")
	{
		admin.GET("/", adminIndex)
		admin.GET("/users", adminUsers)
		admin.POST("/user/status", userStatus)
	}

	// Oauth2
	o := r.Group("oauth2")
	{
		// Authorization code endpoint
		o.GET("auth", oauth2auth)
		// Access token endpoint
		o.GET("token", oauth2token)
		o.GET("info", oauth2info)
		o.GET("publickeys", openIDConnectPublickeys)

		// OpenIDConnect
		r.GET("/.well-known/openid-configuration", openIDConnectDiscovery)
	}

	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "page/info", gin.H{
			"title": "无法找到页面",
			"icon":  "rocket",
			"msg":   "页面可能已飞去火星",
		})
	})
	r.NoMethod(func(c *gin.Context) {
		c.HTML(http.StatusForbidden, "page/info", gin.H{
			"title": "发现新大陆",
			"icon":  "paw",
			"msg":   "没有这个请求方式哦",
		})
	})
	r.Run(":8080")
}

func genClientID(uid string) (id string, err error) {
	for i := 0; i < 100; i++ {
		id = uid + "-" + com.RandomString(6)
		if _, err = osinStore.GetClient(id); err == nil {
			continue
		}
		return id, nil
	}
	return "", errors.New("genClientID 重试次数达到限制。")
}
