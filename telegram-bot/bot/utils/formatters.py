"""
Утилиты для форматирования текста и сообщений
"""

import re
from typing import List, Optional, Dict, Any
from datetime import datetime
import html

class MessageFormatter:
    """Класс для форматирования сообщений"""
    
    @staticmethod
    def format_for_telegram(text: str, parse_mode: Optional[str] = None) -> str:
        """
        Форматирование текста для Telegram с учетом ограничений
        
        Args:
            text: Исходный текст
            parse_mode: Режим парсинга (Markdown, HTML)
        
        Returns:
            Отформатированный текст
        """
        # Очистка от лишних пробелов
        text = re.sub(r'\s+', ' ', text).strip()
        
        # Обрезаем до максимальной длины Telegram
        if len(text) > 4096:
            text = text[:4090] + "..."
        
        if parse_mode == "Markdown":
            return MessageFormatter._escape_markdown(text)
        elif parse_mode == "HTML":
            return MessageFormatter._escape_html(text)
        else:
            return text
    
    @staticmethod
    def _escape_markdown(text: str) -> str:
        """Экранирование специальных символов Markdown"""
        escape_chars = r'_*[]()~`>#+-=|{}.!'
        for char in escape_chars:
            text = text.replace(char, f'\\{char}')
        return text
    
    @staticmethod
    def _escape_html(text: str) -> str:
        """Экранирование HTML символов"""
        return html.escape(text)
    
    @staticmethod
    def format_ticket_response(
        ai_response: Optional[str] = None,
        operator_response: Optional[str] = None,
        confidence: Optional[float] = None,
        sources: Optional[List[str]] = None
    ) -> str:
        """
        Форматирование ответа на тикет для отправки клиенту
        
        Args:
            ai_response: Ответ от AI
            operator_response: Ответ от оператора
            confidence: Уровень уверенности AI
            sources: Источники информации
        
        Returns:
            Отформатированный ответ
        """
        if operator_response:
            # Ответ от оператора имеет приоритет
            return operator_response
        
        if ai_response:
            lines = [ai_response]
            
            # Добавляем метаинформацию если нужно
            if confidence and confidence < 0.7:
                lines.append("\n⚠️ *Примечание:* Ответ сгенерирован автоматически.")
            
            if sources and "internet" in sources:
                lines.append("\n🌐 *Источник:* информация из открытых источников")
            
            return "\n".join(lines)
        
        return "Спасибо за обращение! Ваш вопрос получен."
    
    @staticmethod
    def format_error_message(error: str, user_friendly: bool = True) -> str:
        """
        Форматирование сообщения об ошибке
        
        Args:
            error: Текст ошибки
            user_friendly: Показывать ли пользователю детали
        
        Returns:
            Отформатированное сообщение об ошибке
        """
        if user_friendly:
            # Пользовательские сообщения
            error_messages = {
                "timeout": "⏳ Время ожидания ответа истекло. Попробуйте еще раз.",
                "network": "📡 Проблемы с соединением. Проверьте интернет.",
                "rate_limit": "⏰ Слишком много запросов. Подождите немного.",
                "maintenance": "🔧 Система на обслуживании. Попробуйте позже.",
                "invalid_input": "❌ Непонятный запрос. Попробуйте переформулировать.",
            }
            
            for key, message in error_messages.items():
                if key in error.lower():
                    return message
            
            # Общее сообщение по умолчанию
            return "😔 Произошла ошибка. Пожалуйста, попробуйте еще раз."
        
        else:
            # Для логирования/отладки
            return f"Ошибка: {error}"
    
    @staticmethod
    def format_stats(stats: Dict[str, Any]) -> str:
        """
        Форматирование статистики в читаемый вид
        
        Args:
            stats: Словарь со статистикой
        
        Returns:
            Отформатированная статистика
        """
        lines = ["📊 *Статистика бота:*", ""]
        
        if "queue_size" in stats:
            lines.append(f"📨 Очередь: {stats['queue_size']} сообщений")
        
        if "total_messages" in stats:
            lines.append(f"📩 Всего сообщений: {stats['total_messages']}")
        
        if "success_rate" in stats:
            success_rate = stats['success_rate'] * 100
            lines.append(f"✅ Успешных отправок: {success_rate:.1f}%")
        
        if "avg_response_time" in stats:
            lines.append(f"⏱ Среднее время ответа: {stats['avg_response_time']:.1f} сек")
        
        if "by_hour" in stats:
            # Пиковый час
            peak_hour = max(stats['by_hour'].items(), key=lambda x: x[1], default=(None, 0))
            if peak_hour[0] is not None:
                lines.append(f"📈 Пиковый час: {peak_hour[0]}:00 ({peak_hour[1]} сообщ.)")
        
        return "\n".join(lines)
    
    @staticmethod
    def truncate_text(text: str, max_length: int = 100, suffix: str = "...") -> str:
        """Обрезка текста с добавлением суффикса"""
        if len(text) <= max_length:
            return text
        
        return text[:max_length - len(suffix)] + suffix
    
    @staticmethod
    def format_timestamp(timestamp: datetime, format_str: str = "%d.%m.%Y %H:%M") -> str:
        """Форматирование временной метки"""
        return timestamp.strftime(format_str)
    
    @staticmethod
    def format_duration(seconds: float) -> str:
        """Форматирование длительности"""
        if seconds < 60:
            return f"{seconds:.1f} сек"
        elif seconds < 3600:
            minutes = seconds / 60
            return f"{minutes:.1f} мин"
        else:
            hours = seconds / 3600
            return f"{hours:.1f} час"


class TextSanitizer:
    """Класс для очистки и валидации текста"""
    
    @staticmethod
    def sanitize_user_input(text: str) -> str:
        """
        Очистка пользовательского ввода от потенциально опасных символов
        
        Args:
            text: Исходный текст
        
        Returns:
            Очищенный текст
        """
        # Удаляем HTML теги
        text = re.sub(r'<[^>]+>', '', text)
        
        # Удаляем слишком длинные последовательности пробелов
        text = re.sub(r'\s{2,}', ' ', text)
        
        # Удаляем управляющие символы (кроме переносов строк)
        text = re.sub(r'[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]', '', text)
        
        # Ограничиваем длину
        text = text[:5000]
        
        return text.strip()
    
    @staticmethod
    def contains_url(text: str) -> bool:
        """Проверяет, содержит ли текст URL"""
        url_pattern = r'https?://(?:[-\w.]|(?:%[\da-fA-F]{2}))+'
        return bool(re.search(url_pattern, text))
    
    @staticmethod
    def extract_urls(text: str) -> List[str]:
        """Извлекает все URL из текста"""
        url_pattern = r'https?://(?:[-\w.]|(?:%[\da-fA-F]{2}))+'
        return re.findall(url_pattern, text)
    
    @staticmethod
    def remove_urls(text: str) -> str:
        """Удаляет все URL из текста"""
        url_pattern = r'https?://(?:[-\w.]|(?:%[\da-fA-F]{2}))+'
        return re.sub(url_pattern, '[ссылка]', text)
    
    @staticmethod
    def normalize_whitespace(text: str) -> str:
        """Нормализация пробельных символов"""
        # Заменяем все пробельные символы на обычные пробелы
        text = re.sub(r'\s+', ' ', text)
        return text.strip()


class EmojiFormatter:
    """Класс для работы с эмодзи"""
    
    # Категории эмодзи
    STATUS_EMOJIS = {
        "success": "✅",
        "error": "❌",
        "warning": "⚠️",
        "info": "ℹ️",
        "question": "❓",
        "loading": "⏳",
        "done": "🎉",
        "clock": "🕒",
        "calendar": "📅",
    }
    
    CHANNEL_EMOJIS = {
        "telegram": "📱",
        "email": "📧",
        "web": "🌐",
        "phone": "📞",
    }
    
    TICKET_EMOJIS = {
        "new": "🆕",
        "processing": "🔄",
        "resolved": "✅",
        "failed": "❌",
        "pending": "⏸️",
    }
    
    @staticmethod
    def add_status_emoji(text: str, status: str) -> str:
        """Добавляет эмодзи статуса к тексту"""
        emoji = EmojiFormatter.STATUS_EMOJIS.get(status.lower(), "")
        return f"{emoji} {text}" if emoji else text
    
    @staticmethod
    def add_channel_emoji(text: str, channel: str) -> str:
        """Добавляет эмодзи канала к тексту"""
        emoji = EmojiFormatter.CHANNEL_EMOJIS.get(channel.lower(), "")
        return f"{emoji} {text}" if emoji else text
    
    @staticmethod
    def add_ticket_emoji(text: str, ticket_status: str) -> str:
        """Добавляет эмодзи статуса тикета к тексту"""
        emoji = EmojiFormatter.TICKET_EMOJIS.get(ticket_status.lower(), "")
        return f"{emoji} {text}" if emoji else text
    
    @staticmethod
    def format_with_emojis(text: str, emoji_map: Dict[str, str]) -> str:
        """
        Заменяет ключевые слова в тексте на эмодзи
        
        Args:
            text: Исходный текст
            emoji_map: Словарь {ключевое_слово: эмодзи}
        
        Returns:
            Текст с эмодзи
        """
        for keyword, emoji in emoji_map.items():
            # Ищем слово целиком
            pattern = rf'\b{re.escape(keyword)}\b'
            text = re.sub(pattern, f'{emoji} {keyword}', text, flags=re.IGNORECASE)
        
        return text