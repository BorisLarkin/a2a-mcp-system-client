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

async def fetch_api_key(local_api_url: str) -> tuple[str, str]:
    """Получает API-ключ и dispatcher_id из local-proxy"""
    async with httpx.AsyncClient(timeout=10.0) as client:
        resp = await client.get(
            f"{local_api_url}/api/v1/public/bot-key",
            headers={"X-Setup-Secret": "internal_setup_secret_123"}
        )
        if resp.status_code == 200:
            data = resp.json()
            return data.get("api_key", ""), data.get("dispatcher_id", "")
    return "", ""

# Синглтон конфига (для обратной совместимости со старыми handlers)
config: Optional[Config] = None