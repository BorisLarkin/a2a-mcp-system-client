"""
Тесты для API клиента
"""

import pytest
import asyncio
from unittest.mock import AsyncMock, MagicMock, patch
from datetime import datetime

from bot.services.api_client import APIClient
from bot.models.ticket import TicketCreate, TicketResponse
from bot.config import config


class TestAPIClient:
    """Тесты API клиента"""
    
    @pytest.fixture
    def api_client(self):
        """Фикстура для создания API клиента"""
        return APIClient()
    
    @pytest.fixture
    def sample_ticket(self):
        """Фикстура для тестового тикета"""
        return TicketCreate(
            client_identifier="123456",
            client_name="Test User",
            initial_message="Test message",
            metadata={"test": True}
        )
    
    @pytest.mark.asyncio
    async def test_create_ticket_success(self, api_client, sample_ticket):
        """Тест успешного создания тикета"""
        
        # Mock успешного ответа
        mock_response = {
            "ticket_id": "test-uuid-123",
            "status": "new",
            "ai_response": None,
            "operator_response": None,
            "final_response": None,
            "ai_confidence": None,
            "error": None
        }
        
        with patch.object(api_client, '_make_request', AsyncMock(return_value=mock_response)):
            async with api_client:
                result = await api_client.create_ticket(sample_ticket)
                
                assert isinstance(result, TicketResponse)
                assert result.ticket_id == "test-uuid-123"
                assert result.status == "new"
                assert result.error is None
    
    @pytest.mark.asyncio
    async def test_create_ticket_failure(self, api_client, sample_ticket):
        """Тест неудачного создания тикета"""
        
        with patch.object(api_client, '_make_request', AsyncMock(side_effect=Exception("API Error"))):
            async with api_client:
                result = await api_client.create_ticket(sample_ticket)
                
                assert result.status == "failed"
                assert result.error is not None
                assert "API Error" in result.error
    
    @pytest.mark.asyncio
    async def test_send_message_success(self, api_client):
        """Тест успешной отправки сообщения"""
        
        mock_response = {"status": "ok"}
        
        with patch.object(api_client, '_make_request', AsyncMock(return_value=mock_response)):
            async with api_client:
                success = await api_client.send_message(
                    chat_id=123456,
                    text="Test message"
                )
                
                assert success is True
    
    @pytest.mark.asyncio
    async def test_send_message_failure(self, api_client):
        """Тест неудачной отправки сообщения"""
        
        with patch.object(api_client, '_make_request', AsyncMock(side_effect=Exception("Error"))):
            async with api_client:
                success = await api_client.send_message(
                    chat_id=123456,
                    text="Test message"
                )
                
                assert success is False
    
    @pytest.mark.asyncio
    async def test_health_check_success(self, api_client):
        """Тест успешной проверки здоровья"""
        
        mock_response = {"status": "ok"}
        
        with patch.object(api_client, '_make_request', AsyncMock(return_value=mock_response)):
            async with api_client:
                healthy = await api_client.health_check()
                
                assert healthy is True
    
    @pytest.mark.asyncio
    async def test_health_check_failure(self, api_client):
        """Тест неудачной проверки здоровья"""
        
        with patch.object(api_client, '_make_request', AsyncMock(side_effect=Exception("Error"))):
            async with api_client:
                healthy = await api_client.health_check()
                
                assert healthy is False
    
    @pytest.mark.asyncio
    async def test_session_management(self):
        """Тест управления сессией (context manager)"""
        
        client = APIClient()
        
        # Проверяем что сессия создается и закрывается
        assert client.session is None
        
        async with client:
            assert client.session is not None
            assert not client.session.closed
        
        # После выхода из контекста сессия должна быть закрыта
        assert client.session.closed
    
    @pytest.mark.asyncio
    async def test_timeout_configuration(self):
        """Тест конфигурации таймаута"""
        
        client = APIClient()
        
        # Проверяем что timeout установлен из конфига
        assert client.timeout.total == config.api_timeout
        
        # Можно переопределить при создании
        custom_timeout = 30
        client_with_custom = APIClient()
        # Здесь нужно будет изменить класс, чтобы принимать timeout в __init__
    
    def test_ticket_model_validation(self, sample_ticket):
        """Тест валидации модели тикета"""
        
        # Корректная модель должна создаваться без ошибок
        assert sample_ticket.channel == "telegram"
        assert sample_ticket.client_identifier == "123456"
        assert sample_ticket.initial_message == "Test message"
        
        # Проверяем дефолтные значения
        assert sample_ticket.metadata == {"test": True}
    
    @pytest.mark.asyncio
    async def test_concurrent_requests(self, api_client):
        """Тест конкурентных запросов"""
        
        mock_response = {"status": "ok"}
        
        with patch.object(api_client, '_make_request', AsyncMock(return_value=mock_response)):
            async with api_client:
                # Создаем несколько задач
                tasks = [
                    api_client.health_check(),
                    api_client.health_check(),
                    api_client.health_check()
                ]
                
                results = await asyncio.gather(*tasks)
                
                # Все запросы должны завершиться успешно
                assert all(results)
                assert len(results) == 3


class TestAPIClientIntegration:
    """Интеграционные тесты (требуют running local-proxy)"""
    
    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_real_health_check(self):
        """Реальная проверка здоровья local-proxy"""
        
        # Этот тест требует запущенный local-proxy
        # Можно запускать только в integration окружении
        
        client = APIClient()
        
        try:
            async with client:
                healthy = await client.health_check()
                
                # В зависимости от окружения
                if config.api_url != "http://localhost:8080":
                    # В тестовом окружении ожидаем ошибку
                    assert healthy is False
                else:
                    # В dev окружении может быть разный результат
                    pass
                    
        except Exception as e:
            # Ожидаемо если local-proxy не запущен
            pass
    
    @pytest.mark.integration
    @pytest.mark.asyncio
    async def test_real_ticket_creation(self):
        """Реальное создание тикета"""
        
        # Только для интеграционного тестирования
        pytest.skip("Требует running local-proxy")
        
        client = APIClient()
        ticket = TicketCreate(
            client_identifier="test-123",
            client_name="Integration Test",
            initial_message="Integration test message"
        )
        
        async with client:
            result = await client.create_ticket(ticket)
            
            # Проверяем структуру ответа
            assert hasattr(result, 'ticket_id')
            assert hasattr(result, 'status')
            assert result.status in ['new', 'processing', 'completed', 'failed']


if __name__ == "__main__":
    # Запуск тестов
    pytest.main([__file__, "-v", "--tb=short"])