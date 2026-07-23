package manager

import (
	"net/http"

	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/apperrors"
	"github.com/gin-gonic/gin"
)

type Manager struct {
	router gin.IRouter
}

func New(router gin.IRouter) *Manager {
	return &Manager{router: router}
}

func (m *Manager) Use(middlewares ...gin.HandlerFunc) {
	m.router.Use(middlewares...)
}

func (m *Manager) Register(cmd apphttp.Command) {
	m.router.Handle(cmd.Method(), cmd.Path(), func(ctx *gin.Context) {
		clone := cmd.Clone()

		err := clone.Parse(ctx)
		if err != nil {
			_ = ctx.Error(err)
			apperrors.WriteError(ctx, err)

			return
		}

		response, err := clone.Apply(ctx)
		if err != nil {
			_ = ctx.Error(err)
			apperrors.WriteError(ctx, err)

			return
		}

		ctx.JSON(http.StatusOK, response)
	})
}
