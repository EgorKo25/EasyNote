package router

import (
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/transport/http/commands"
	"github.com/EgorKo25/EasyNote/src/transport/http/manager"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const apiV1Prefix = "/api/v1"

type Factory interface {
	Create() http.Handler
}

type factory struct {
	reminderService *service.ReminderService
}

func NewFactory(reminderService *service.ReminderService) Factory {
	return &factory{
		reminderService: reminderService,
	}
}

func Create(reminderService *service.ReminderService) http.Handler {
	return NewFactory(reminderService).Create()
}

func (f *factory) Create() http.Handler {
	engine := gin.New()
	engine.Use(middleware.CORSMiddleware(), middleware.RequestLoggerMiddleware(), gin.Recovery())

	api := engine.Group(apiV1Prefix)

	publicManager := manager.New(api)
	publicManager.Register(commands.NewGetHealthCommand())
	publicManager.Register(commands.NewGetTimezonesCommand())

	protectedManager := manager.New(api.Group(""))
	protectedManager.Use(middleware.TelegramChatIDMiddleware())
	protectedManager.Register(commands.NewGetRemindersCommand(f.reminderService))
	protectedManager.Register(commands.NewQueryRemindersCommand(f.reminderService))
	protectedManager.Register(commands.NewCreateReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewGetReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewUpdateReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewDeleteReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewCompleteReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewCancelReminderCommand(f.reminderService))
	protectedManager.Register(commands.NewGetHolidaysCommand(f.reminderService))
	protectedManager.Register(commands.NewCreateHolidayCommand(f.reminderService))
	protectedManager.Register(commands.NewGetHolidayCommand(f.reminderService))
	protectedManager.Register(commands.NewUpdateHolidayCommand(f.reminderService))
	protectedManager.Register(commands.NewDeleteHolidayCommand(f.reminderService))
	protectedManager.Register(commands.NewGetShoppingListsCommand(f.reminderService))
	protectedManager.Register(commands.NewCreateShoppingListCommand(f.reminderService))
	protectedManager.Register(commands.NewGetShoppingListCommand(f.reminderService))
	protectedManager.Register(commands.NewToggleShoppingListItemCommand(f.reminderService))
	protectedManager.Register(commands.NewGetDashboardCommand(f.reminderService))
	protectedManager.Register(commands.NewGetSettingsCommand(f.reminderService))
	protectedManager.Register(commands.NewUpdateSettingsCommand(f.reminderService))

	return engine
}
