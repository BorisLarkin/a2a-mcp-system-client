from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Config:
    bot_token: str
    local_api_url: str = "http://local-proxy:8080"
    api_key: str = ""
    dispatcher_id: str = ""
    bot_username: str = "a2a_mcp_company_support_bot"
    offline_response: str = "Спасибо за обращение! Мы получили ваше сообщение и ответим при первой возможности."
    rate_limit_per_user: int = 5
    max_message_length: int = 4000
    maintenance_mode: bool = False
    debug: bool = False


# Синглтон конфига (для обратной совместимости со старыми handlers)
config: Optional[Config] = None