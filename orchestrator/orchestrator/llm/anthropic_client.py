import logging
import time

import anthropic

from ..config import ANTHROPIC_API_KEY, ANTHROPIC_MODEL

logger = logging.getLogger(__name__)

_MAX_ATTEMPTS = 3
_RETRY_DELAYS = [1, 2, 4]
_RETRYABLE = (anthropic.RateLimitError, anthropic.InternalServerError)


class AnthropicClient:
    def __init__(self, api_key: str = ANTHROPIC_API_KEY, model: str = ANTHROPIC_MODEL) -> None:
        self._model = model
        self._client = anthropic.Anthropic(api_key=api_key, max_retries=0)

    def complete(self, prompt: str, system: str) -> str:
        last_err: Exception | None = None
        for attempt in range(_MAX_ATTEMPTS):
            try:
                response = self._client.messages.create(
                    model=self._model,
                    max_tokens=4096,
                    system=system,
                    messages=[{"role": "user", "content": prompt}],
                )
                logger.debug(
                    "llm call model=%s input_tokens=%d output_tokens=%d",
                    self._model,
                    response.usage.input_tokens,
                    response.usage.output_tokens,
                )
                return response.content[0].text
            except _RETRYABLE as exc:
                last_err = exc
                if attempt < _MAX_ATTEMPTS - 1:
                    delay = _RETRY_DELAYS[attempt]
                    logger.warning(
                        "llm retry attempt=%d/%d delay=%ds error=%s",
                        attempt + 1,
                        _MAX_ATTEMPTS,
                        delay,
                        type(exc).__name__,
                    )
                    time.sleep(delay)
        raise last_err  # type: ignore[misc]
