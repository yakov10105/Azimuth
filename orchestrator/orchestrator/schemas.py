from pydantic import BaseModel, Field


class AskRequest(BaseModel):
    question: str
    depth: int = Field(default=3, ge=1, le=10)


class AskResponse(BaseModel):
    summary: str
    call_path: list[str]
    relevant_files: list[str]
