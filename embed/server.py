from contextlib import asynccontextmanager
from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

model = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    global model
    model = SentenceTransformer("all-MiniLM-L6-v2")
    yield

app = FastAPI(lifespan=lifespan)

class EmbedRequest(BaseModel):
    text: str

class EmbedResponse(BaseModel):
    embedding: list[float]

@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest):
    vec = model.encode(req.text).tolist()
    return EmbedResponse(embedding=vec)

@app.get("/health")
def health():
    return {"status": "ok", "model": "all-MiniLM-L6-v2", "dimensions": 384}
