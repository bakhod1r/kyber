package httpx

import "github.com/bakhod1r/errorx"

// Kyber error codes (errorx registry, numeric range 6xxx). Clients branch on Code;
// titles are localized from Accept-Language; detail carries the specific reason.
const (
	CodeBadRequest           = "BAD_REQUEST"
	CodeValidation           = "VALIDATION_FAILED"
	CodeAuthRequired         = "AUTH_REQUIRED"
	CodeInvalidCredentials   = "AUTH_INVALID_CREDENTIALS"
	CodeTooManyAttempts      = "AUTH_TOO_MANY_ATTEMPTS"
	CodeEmailTaken           = "EMAIL_TAKEN"
	CodeCSRF                 = "CSRF_REJECTED"
	CodeProviderDisabled     = "AUTH_PROVIDER_DISABLED"
	CodeOTPInvalid           = "AUTH_OTP_INVALID"
	CodeOTPPending           = "AUTH_OTP_PENDING"
	CodeProjectNotFound      = "PROJECT_NOT_FOUND"
	CodeProjectKeyTaken      = "PROJECT_KEY_TAKEN"
	CodeForbidden            = "PROJECT_FORBIDDEN"
	CodeLastAdmin            = "PROJECT_LAST_ADMIN"
	CodeUserNotFound         = "USER_NOT_FOUND"
	CodeIssueNotFound        = "ISSUE_NOT_FOUND"
	CodeInvalidIssueKey      = "ISSUE_INVALID_KEY"
	CodeTransitionNotAllowed = "ISSUE_TRANSITION_NOT_ALLOWED"
	CodeConcurrentModified   = "ISSUE_CONFLICT"
	CodeSprintNotFound       = "SPRINT_NOT_FOUND"
	CodeSprintState          = "SPRINT_STATE"
	CodeSprintAlreadyActive  = "SPRINT_ALREADY_ACTIVE"
	CodeSprintConflict       = "SPRINT_CONFLICT"
	CodeNotificationNotFound = "NOTIFICATION_NOT_FOUND"
	CodeImportTooLarge       = "IMPORT_TOO_LARGE"
	CodeImportInvalidFile    = "IMPORT_INVALID_FILE"
	CodeInternal             = "INTERNAL"
)

type code struct {
	code, numeric string
	status        int
	en, uz, ru    string
	cat           errorx.ErrorCategory
}

var codes = []code{
	{CodeBadRequest, "6000", 400, "The request could not be read.", "So'rovni o'qib bo'lmadi.", "Не удалось прочитать запрос.", errorx.CategoryValidation},
	{CodeValidation, "6001", 422, "Some fields are invalid.", "Ba'zi maydonlar noto'g'ri.", "Некоторые поля заполнены неверно.", errorx.CategoryValidation},
	{CodeAuthRequired, "6030", 401, "Please log in.", "Iltimos, tizimga kiring.", "Пожалуйста, войдите в систему.", errorx.CategorySecurity},
	{CodeInvalidCredentials, "6031", 401, "Invalid email or password.", "Email yoki parol noto'g'ri.", "Неверный email или пароль.", errorx.CategorySecurity},
	{CodeTooManyAttempts, "6032", 429, "Too many attempts. Try again later.", "Urinishlar juda ko'p. Keyinroq urinib ko'ring.", "Слишком много попыток. Попробуйте позже.", errorx.CategorySecurity},
	{CodeEmailTaken, "6033", 409, "This email is already registered.", "Bu email allaqachon ro'yxatdan o'tgan.", "Этот email уже зарегистрирован.", errorx.CategoryBusiness},
	{CodeCSRF, "6034", 403, "Request rejected by CSRF protection.", "So'rov CSRF himoyasi tomonidan rad etildi.", "Запрос отклонён защитой от CSRF.", errorx.CategorySecurity},
	{CodeProviderDisabled, "6035", 404, "This sign-in method is not enabled.", "Bu kirish usuli yoqilmagan.", "Этот способ входа не включён.", errorx.CategoryBusiness},
	{CodeOTPInvalid, "6036", 401, "The code is wrong or expired.", "Kod noto'g'ri yoki muddati o'tgan.", "Код неверный или устарел.", errorx.CategorySecurity},
	{CodeOTPPending, "6037", 409, "Open the Telegram bot and press Start to get your code.", "Kodni olish uchun Telegram botni ochib, Start tugmasini bosing.", "Откройте Telegram-бота и нажмите Start, чтобы получить код.", errorx.CategoryBusiness},
	{CodeProjectNotFound, "6002", 404, "Project not found.", "Loyiha topilmadi.", "Проект не найден.", errorx.CategoryBusiness},
	{CodeProjectKeyTaken, "6003", 409, "This project key is already taken.", "Bu loyiha kaliti band.", "Этот ключ проекта уже занят.", errorx.CategoryBusiness},
	{CodeForbidden, "6004", 403, "Your project role does not allow this.", "Loyihadagi rolingiz bunga ruxsat bermaydi.", "Ваша роль в проекте не позволяет это сделать.", errorx.CategorySecurity},
	{CodeLastAdmin, "6005", 409, "A project must keep at least one admin.", "Loyihada kamida bitta admin qolishi kerak.", "В проекте должен остаться хотя бы один админ.", errorx.CategoryBusiness},
	{CodeUserNotFound, "6006", 404, "No user with that email.", "Bunday emailga ega foydalanuvchi yo'q.", "Пользователь с таким email не найден.", errorx.CategoryBusiness},
	{CodeIssueNotFound, "6010", 404, "Issue not found.", "Vazifa topilmadi.", "Задача не найдена.", errorx.CategoryBusiness},
	{CodeInvalidIssueKey, "6011", 400, "Invalid issue key.", "Vazifa kaliti noto'g'ri.", "Неверный ключ задачи.", errorx.CategoryValidation},
	{CodeTransitionNotAllowed, "6012", 409, "This status change is not allowed.", "Bu holatga o'tkazib bo'lmaydi.", "Такой переход статуса запрещён.", errorx.CategoryBusiness},
	{CodeConcurrentModified, "6013", 409, "Someone else changed this issue. Reload and try again.", "Vazifani boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Задачу изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeSprintNotFound, "6020", 404, "Sprint not found.", "Sprint topilmadi.", "Спринт не найден.", errorx.CategoryBusiness},
	{CodeSprintState, "6021", 409, "The sprint is not in a state that allows this.", "Sprint holati bunga ruxsat bermaydi.", "Текущее состояние спринта этого не допускает.", errorx.CategoryBusiness},
	{CodeSprintAlreadyActive, "6022", 409, "This project already has an active sprint.", "Bu loyihada allaqachon faol sprint bor.", "В этом проекте уже есть активный спринт.", errorx.CategoryBusiness},
	{CodeSprintConflict, "6023", 409, "Someone else changed this sprint. Reload and try again.", "Sprintni boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Спринт изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeNotificationNotFound, "6040", 404, "Notification not found.", "Bildirishnoma topilmadi.", "Уведомление не найдено.", errorx.CategoryBusiness},
	{CodeImportTooLarge, "6050", 413, "The file is too large to import (10 MB max).", "Fayl import uchun juda katta (ko'pi bilan 10 MB).", "Файл слишком большой для импорта (не более 10 МБ).", errorx.CategoryValidation},
	{CodeImportInvalidFile, "6051", 422, "This is not a Jira CSV export.", "Bu Jira CSV eksport fayli emas.", "Это не CSV-экспорт из Jira.", errorx.CategoryValidation},
	{CodeInternal, "6099", 500, "Something went wrong.", "Nimadir noto'g'ri ketdi.", "Что-то пошло не так.", errorx.CategorySystem},
}

func init() {
	defs := make([]errorx.CustomError, 0, len(codes))
	for _, c := range codes {
		defs = append(defs, errorx.CustomError{
			Code: c.code, Numeric: c.numeric, HTTPStatus: c.status, Category: c.cat,
			Severity: errorx.SeverityLow, Layer: errorx.LayerHandler,
			Message: errorx.UserMessage{En: c.en, Uz: c.uz, Ru: c.ru},
		})
	}
	errorx.MustRegisterErrors(defs...)
}
