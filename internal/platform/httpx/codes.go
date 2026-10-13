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
	CodeWorkspaceNotFound    = "WORKSPACE_NOT_FOUND"
	CodeFocusMeeting         = "FOCUS_MEETING_CONFLICT"
	CodeFocusRunning         = "FOCUS_ALREADY_RUNNING"
	CodeFocusNotFound        = "FOCUS_NOT_FOUND"
	CodeFocusState           = "FOCUS_STATE"
	CodeMeetingNotFound      = "MEETING_NOT_FOUND"
	CodeMeetingForbidden     = "MEETING_FORBIDDEN"
	CodeConversationNotFound = "CONVERSATION_NOT_FOUND"
	CodeMessageNotFound      = "MESSAGE_NOT_FOUND"
	CodeChannelNameTaken     = "CHANNEL_NAME_TAKEN"
	CodeChatForbidden        = "CHAT_FORBIDDEN"
	CodeCallNotFound         = "CALL_NOT_FOUND"
	CodeCallFull             = "CALL_FULL"
	CodeCallState            = "CALL_STATE"
	CodeWorkspaceSlugTaken   = "WORKSPACE_SLUG_TAKEN"
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
	CodeStatusInUse          = "WORKFLOW_STATUS_IN_USE"
	CodeInvalidParent        = "ISSUE_INVALID_PARENT"
	CodeInvalidLink          = "ISSUE_LINK_INVALID"
	CodeLinkExists           = "ISSUE_LINK_EXISTS"
	CodeLinkNotFound         = "ISSUE_LINK_NOT_FOUND"
	CodeSprintNotFound       = "SPRINT_NOT_FOUND"
	CodeSprintState          = "SPRINT_STATE"
	CodeSprintAlreadyActive  = "SPRINT_ALREADY_ACTIVE"
	CodeSprintConflict       = "SPRINT_CONFLICT"
	CodeBoardNotFound        = "BOARD_NOT_FOUND"
	CodeBoardConflict        = "BOARD_CONFLICT"
	CodeLastBoard            = "BOARD_LAST"
	CodeVersionNotFound      = "VERSION_NOT_FOUND"
	CodeVersionConflict      = "VERSION_CONFLICT"
	CodeVersionState         = "VERSION_STATE"
	CodeVersionNameTaken     = "VERSION_NAME_TAKEN"
	CodeWorklogNotFound      = "WORKLOG_NOT_FOUND"
	CodeInvalidJQL           = "SEARCH_INVALID_JQL"
	CodeFilterNotFound       = "FILTER_NOT_FOUND"
	CodeFilterConflict       = "FILTER_CONFLICT"
	CodeFieldNotFound        = "FIELD_NOT_FOUND"
	CodeFieldConflict        = "FIELD_CONFLICT"
	CodeNotificationNotFound = "NOTIFICATION_NOT_FOUND"
	CodeImportTooLarge       = "IMPORT_TOO_LARGE"
	CodeImportInvalidFile    = "IMPORT_INVALID_FILE"
	CodeNotFound             = "NOT_FOUND"
	CodeMethodNotAllowed     = "METHOD_NOT_ALLOWED"
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
	{CodeWorkspaceNotFound, "6060", 404, "Workspace not found.", "Workspace topilmadi.", "Рабочее пространство не найдено.", errorx.CategoryBusiness},
	{CodeWorkspaceSlugTaken, "6061", 409, "This workspace address is already taken.", "Bu workspace manzili band.", "Этот адрес рабочего пространства уже занят.", errorx.CategoryBusiness},
	{CodeFocusMeeting, "6070", 409, "You have a meeting at that time.", "Bu vaqtda sizda uchrashuv bor.", "В это время у вас встреча.", errorx.CategoryBusiness},
	{CodeFocusRunning, "6071", 409, "You already have a focus session.", "Sizda allaqachon fokus seansi bor.", "У вас уже идёт фокус-сессия.", errorx.CategoryBusiness},
	{CodeFocusNotFound, "6072", 404, "No focus session is running.", "Fokus seansi ishlamayapti.", "Фокус-сессия не запущена.", errorx.CategoryBusiness},
	{CodeFocusState, "6073", 409, "The focus session cannot do that now.", "Fokus seansi hozir buni qila olmaydi.", "Фокус-сессия сейчас не может этого сделать.", errorx.CategoryBusiness},
	{CodeMeetingNotFound, "6074", 404, "Meeting not found.", "Uchrashuv topilmadi.", "Встреча не найдена.", errorx.CategoryBusiness},
	{CodeMeetingForbidden, "6075", 403, "Only the organizer can cancel this meeting.", "Uchrashuvni faqat tashkilotchi bekor qila oladi.", "Отменить встречу может только организатор.", errorx.CategorySecurity},
	{CodeConversationNotFound, "6100", 404, "Conversation not found.", "Suhbat topilmadi.", "Беседа не найдена.", errorx.CategoryBusiness},
	{CodeMessageNotFound, "6101", 404, "Message not found.", "Xabar topilmadi.", "Сообщение не найдено.", errorx.CategoryBusiness},
	{CodeChannelNameTaken, "6102", 409, "A channel with this name already exists.", "Bu nomli kanal allaqachon bor.", "Канал с таким названием уже существует.", errorx.CategoryBusiness},
	{CodeCallNotFound, "6104", 404, "Call not found.", "Qo'ng'iroq topilmadi.", "Звонок не найден.", errorx.CategoryBusiness},
	{CodeCallFull, "6105", 409, "The call is full (6 people at most).", "Qo'ng'iroq to'lgan (ko'pi bilan 6 kishi).", "Звонок заполнен (не более 6 человек).", errorx.CategoryBusiness},
	{CodeCallState, "6106", 409, "You are not in this call.", "Siz bu qo'ng'iroqda emassiz.", "Вы не участвуете в этом звонке.", errorx.CategoryBusiness},
	{CodeChatForbidden, "6103", 403, "You cannot do that in this conversation.", "Bu suhbatda buni qila olmaysiz.", "В этой беседе так сделать нельзя.", errorx.CategorySecurity},
	{CodeProjectNotFound, "6002", 404, "Project not found.", "Loyiha topilmadi.", "Проект не найден.", errorx.CategoryBusiness},
	{CodeProjectKeyTaken, "6003", 409, "This project key is already taken.", "Bu loyiha kaliti band.", "Этот ключ проекта уже занят.", errorx.CategoryBusiness},
	{CodeForbidden, "6004", 403, "Your project role does not allow this.", "Loyihadagi rolingiz bunga ruxsat bermaydi.", "Ваша роль в проекте не позволяет это сделать.", errorx.CategorySecurity},
	{CodeLastAdmin, "6005", 409, "A project must keep at least one admin.", "Loyihada kamida bitta admin qolishi kerak.", "В проекте должен остаться хотя бы один админ.", errorx.CategoryBusiness},
	{CodeUserNotFound, "6006", 404, "No user with that email.", "Bunday emailga ega foydalanuvchi yo'q.", "Пользователь с таким email не найден.", errorx.CategoryBusiness},
	{CodeIssueNotFound, "6010", 404, "Issue not found.", "Vazifa topilmadi.", "Задача не найдена.", errorx.CategoryBusiness},
	{CodeInvalidIssueKey, "6011", 400, "Invalid issue key.", "Vazifa kaliti noto'g'ri.", "Неверный ключ задачи.", errorx.CategoryValidation},
	{CodeTransitionNotAllowed, "6012", 409, "This status change is not allowed.", "Bu holatga o'tkazib bo'lmaydi.", "Такой переход статуса запрещён.", errorx.CategoryBusiness},
	{CodeConcurrentModified, "6013", 409, "Someone else changed this issue. Reload and try again.", "Vazifani boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Задачу изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeInvalidParent, "6015", 422, "That parent is not allowed: epics hold stories, tasks and bugs; those hold sub-tasks.", "Bunday ota vazifa mumkin emas: epic ichida story, task va bug; ularning ichida sub-task bo'ladi.", "Такой родитель недопустим: эпик содержит истории, задачи и баги, а они — подзадачи.", errorx.CategoryValidation},
	{CodeInvalidLink, "6016", 422, "Choose blocks, relates, duplicates or clones, between two different issues.", "Blocks, relates, duplicates yoki clones turini tanlang; ikki xil vazifa bo'lishi kerak.", "Выберите blocks, relates, duplicates или clones между двумя разными задачами.", errorx.CategoryValidation},
	{CodeLinkExists, "6017", 409, "These issues are already linked this way.", "Bu vazifalar allaqachon shu tarzda bog'langan.", "Эти задачи уже связаны таким образом.", errorx.CategoryBusiness},
	{CodeLinkNotFound, "6018", 404, "Link not found.", "Bog'lanish topilmadi.", "Связь не найдена.", errorx.CategoryBusiness},
	{CodeStatusInUse, "6014", 409, "Issues still use this status. Move them first.", "Bu holatda hali vazifalar bor. Avval ularni boshqa holatga o'tkazing.", "В этом статусе ещё есть задачи. Сначала перенесите их.", errorx.CategoryBusiness},
	{CodeSprintNotFound, "6020", 404, "Sprint not found.", "Sprint topilmadi.", "Спринт не найден.", errorx.CategoryBusiness},
	{CodeSprintState, "6021", 409, "The sprint is not in a state that allows this.", "Sprint holati bunga ruxsat bermaydi.", "Текущее состояние спринта этого не допускает.", errorx.CategoryBusiness},
	{CodeSprintAlreadyActive, "6022", 409, "This project already has an active sprint.", "Bu loyihada allaqachon faol sprint bor.", "В этом проекте уже есть активный спринт.", errorx.CategoryBusiness},
	{CodeSprintConflict, "6023", 409, "Someone else changed this sprint. Reload and try again.", "Sprintni boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Спринт изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeBoardNotFound, "6024", 404, "Board not found.", "Board topilmadi.", "Доска не найдена.", errorx.CategoryBusiness},
	{CodeBoardConflict, "6025", 409, "Someone else changed this board. Reload and try again.", "Boardni boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Доску изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeVersionNotFound, "6080", 404, "Version not found.", "Versiya topilmadi.", "Версия не найдена.", errorx.CategoryBusiness},
	{CodeVersionConflict, "6081", 409, "Someone else changed this version. Reload and try again.", "Versiyani boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Версию изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeVersionState, "6082", 409, "The version is not in a state that allows this.", "Versiya holati bunga ruxsat bermaydi.", "Текущее состояние версии этого не допускает.", errorx.CategoryBusiness},
	{CodeVersionNameTaken, "6083", 409, "This project already has a version with that name.", "Loyihada bu nomli versiya allaqachon bor.", "В проекте уже есть версия с таким названием.", errorx.CategoryBusiness},
	{CodeWorklogNotFound, "6084", 404, "Worklog not found.", "Vaqt yozuvi topilmadi.", "Запись о работе не найдена.", errorx.CategoryBusiness},
	{CodeInvalidJQL, "6090", 422, "The query is not valid JQL.", "So'rov JQL qoidalariga mos emas.", "Запрос не является корректным JQL.", errorx.CategoryValidation},
	{CodeFilterNotFound, "6091", 404, "Filter not found.", "Filtr topilmadi.", "Фильтр не найден.", errorx.CategoryBusiness},
	{CodeFilterConflict, "6092", 409, "Someone else changed this filter. Reload and try again.", "Filtrni boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Фильтр изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeFieldNotFound, "6093", 404, "Custom field not found.", "Maxsus maydon topilmadi.", "Пользовательское поле не найдено.", errorx.CategoryBusiness},
	{CodeFieldConflict, "6094", 409, "Another field has this name, or someone else changed it. Reload and try again.", "Bu nomli maydon bor yoki uni boshqa kishi o'zgartirdi. Qayta yuklab, yana urinib ko'ring.", "Поле с таким именем уже есть или его изменил кто-то другой. Обновите и попробуйте снова.", errorx.CategoryBusiness},
	{CodeLastBoard, "6026", 409, "A project must keep at least one board.", "Loyihada kamida bitta board qolishi kerak.", "В проекте должна остаться хотя бы одна доска.", errorx.CategoryBusiness},
	{CodeNotificationNotFound, "6040", 404, "Notification not found.", "Bildirishnoma topilmadi.", "Уведомление не найдено.", errorx.CategoryBusiness},
	{CodeImportTooLarge, "6050", 413, "The file is too large to import (10 MB max).", "Fayl import uchun juda katta (ko'pi bilan 10 MB).", "Файл слишком большой для импорта (не более 10 МБ).", errorx.CategoryValidation},
	{CodeImportInvalidFile, "6051", 422, "This is not a Jira CSV export.", "Bu Jira CSV eksport fayli emas.", "Это не CSV-экспорт из Jira.", errorx.CategoryValidation},
	{CodeNotFound, "6097", 404, "No such API endpoint.", "Bunday API manzili yo'q.", "Такого адреса API нет.", errorx.CategoryValidation},
	{CodeMethodNotAllowed, "6098", 405, "This method is not allowed here.", "Bu yerda bu metodga ruxsat yo'q.", "Этот метод здесь не разрешён.", errorx.CategoryValidation},
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
