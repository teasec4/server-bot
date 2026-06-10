package telegram

type sendMessageRequest struct {
	ChatID                string               `json:"chat_id"`
	Text                  string               `json:"text"`
	DisableWebPagePreview bool                 `json:"disable_web_page_preview"`
	ReplyMarkup           *replyKeyboardMarkup `json:"reply_markup,omitempty"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

type getUpdatesRequest struct {
	Offset         int      `json:"offset,omitempty"`
	Timeout        int      `json:"timeout"`
	AllowedUpdates []string `json:"allowed_updates"`
}

type getUpdatesResponse struct {
	OK          bool     `json:"ok"`
	Result      []update `json:"result"`
	Description string   `json:"description"`
}

type update struct {
	UpdateID int      `json:"update_id"`
	Message  *message `json:"message,omitempty"`
}

type message struct {
	Text string `json:"text"`
	Chat chat   `json:"chat"`
	From *user  `json:"from,omitempty"`
}

type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type,omitempty"`
}

type user struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type replyKeyboardMarkup struct {
	Keyboard       [][]keyboardButton `json:"keyboard"`
	ResizeKeyboard bool               `json:"resize_keyboard"`
}

type keyboardButton struct {
	Text string `json:"text"`
}
