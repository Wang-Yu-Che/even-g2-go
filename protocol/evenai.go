package protocol

const (
	EvenAIStatusWakeUp = 1
	EvenAIStatusEnter  = 2
	EvenAIStatusExit   = 3

	EvenAISkillBrightness     = 1
	EvenAISkillTranslation    = 2
	EvenAISkillNotification   = 3
	EvenAISkillTeleprompter   = 4
	EvenAISkillNavigation     = 5
	EvenAISkillConversation   = 6
	EvenAISkillQuicklist      = 7
	EvenAISkillAutoBrightness = 8
)

func BuildEvenAIConfig(magic int, enabled bool) []byte {
	config := []byte(nil)
	if enabled {
		config = protoUint(1, 1)
	}
	config = append(config, protoUint(2, 32)...)
	payload := protoUint(1, 10)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(13, config)...)
}

func BuildEvenAIControl(magic, status int) []byte {
	payload := protoUint(1, 1)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(3, protoUint(1, status))...)
}

func BuildEvenAIAsk(magic int, text string, streamEnabled bool) []byte {
	ask := protoUintPresent(2, boolInt(streamEnabled))
	ask = append(ask, protoBytes(4, []byte(text))...)
	payload := protoUint(1, 3)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(5, ask)...)
}

func BuildEvenAISkill(magic, skillID, skillParam int, text string) []byte {
	skill := protoUint(1, 1)
	skill = append(skill, protoUint(2, skillID)...)
	skill = append(skill, protoUint(3, skillParam)...)
	skill = append(skill, protoBytes(4, []byte(text))...)
	skill = append(skill, protoUint(6, 1)...)
	payload := protoUint(1, 6)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(8, skill)...)
}
