from typing import Protocol


class LLMClient(Protocol):
    def complete(self, prompt: str, system: str) -> str: ...
