package handlers

import (
	tele "gopkg.in/telebot.v4"
)

var (
	// Initialize the menu structure
	LangMenu = &tele.ReplyMarkup{}
	
	// Declare variables for the buttons, but initialize them in init()
	BtnRu tele.Btn
	BtnEn tele.Btn
)


func init() {
	// Buttons need to be created using the methods of the LangMenu itself (the *ReplyMarkup object)
	BtnRu = LangMenu.Data("RU Русский", "set_lang_ru")
	BtnEn = LangMenu.Data("GB English", "set_lang_en")

	// Build the inline menuню
	LangMenu.Inline(
		LangMenu.Row(BtnRu, BtnEn),
	)
}
