# Query Pipeline Flow - Visual

## The Complete Journey

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                               │
│  USER TYPES QUESTION                                                         │
│  "How does the PDF explain pump maintenance?"                               │
│                                                                               │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  TOKENIZATION                                                                │
│  Break into tokens: [How] [does] [the] [PDF] [explain] [pump] [maintenance] │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  VECTORIZATION (Embedding)                                                   │
│  Convert to 768 numbers using Ollama gemma4:e2b                             │
│  [0.123, -0.456, 0.789, ... (765 more numbers)]                            │
│  ✓ Numbers represent "meaning" of the question                              │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  VECTOR SEARCH (Pinecone)                                                    │
│  Find 3 most similar document chunks in database                            │
│                                                                               │
│  Match 1: Score 0.998 (99.8% similar)                                       │
│  Match 2: Score 0.876 (87.6% similar)                                       │
│  Match 3: Score 0.755 (75.5% similar)                                       │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  RETRIEVE CHUNKS WITH METADATA                                               │
│                                                                               │
│  Chunk 1: "Pump maintenance involves checking pressure gauge..."            │
│           Page: 45 | Chapter: 3 | Score: 99.8%                             │
│                                                                               │
│  Chunk 2: "Regular maintenance reduces downtime by 40%..."                  │
│           Page: 51 | Chapter: 4 | Score: 87.6%                             │
│                                                                               │
│  Chunk 3: "Tools needed: wrench, gauge, sealant..."                         │
│           Page: 67 | Chapter: 5 | Score: 75.5%                             │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  BUILD RAG PROMPT                                                            │
│                                                                               │
│  System: "Use ONLY the context below to answer"                             │
│  Context: [The 3 chunks above]                                              │
│  Question: "How does the PDF explain pump maintenance?"                     │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  LLM GENERATION (Ollama gemma4:e4b)                                          │
│  Stream tokens one-by-one:                                                  │
│                                                                               │
│  "Based" → "on" → "the" → "provided" → "PDF" → "excerpts"...              │
│  (appears word-by-word in real-time)                                        │
└────────────────────────────┬────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  USER SEES ANSWER                                                            │
│                                                                               │
│  Answer:                                                                     │
│  "Based on the provided PDF excerpts, pump maintenance involves checking    │
│   the pressure gauge, verifying oil levels, and inspecting seals. The       │
│   process requires: 1) Turn off power, 2) Release pressure safely,          │
│   3) Inspect each component for wear or damage. Regular maintenance can     │
│   reduce downtime by up to 40%."                                            │
│                                                                               │
│  Sources:                                                                    │
│  ✓ Source 1: Page 45, Chapter 3 (99.8% match) - [View Full]               │
│  ✓ Source 2: Page 51, Chapter 4 (87.6% match) - [View Full]               │
│  ✓ Source 3: Page 67, Chapter 5 (75.5% match) - [View Full]               │
│                                                                               │
│  Ready for next question...                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## What Happens at Each Stage

### 1️⃣ Tokenization
Your text gets broken into pieces the AI understands

### 2️⃣ Vectorization  
Your question becomes 768 numbers representing its meaning

### 3️⃣ Vector Search
System finds document chunks closest to your question's meaning

### 4️⃣ Chunk Retrieval
Get the actual text + location info from PDFs

### 5️⃣ RAG Prompt
Combine: Instructions + Context + Your Question

### 6️⃣ LLM Generation
AI writes answer based on context, streams it to you

### 7️⃣ Answer + Sources
You get the answer with proof of where it came from

---

## The Two Key Models

| Stage | Model | Purpose |
|-------|-------|---------|
| Vectorization (Step 2) | `gemma4:e2b` | Convert text to vectors |
| Answer Generation (Step 6) | `gemma4:e4b` | Write the answer |

---

## Why This Matters

✅ **Grounded in your documents** - not internet hallucinations  
✅ **Semantic search** - finds meaning, not just keywords  
✅ **Real-time feedback** - answer streams as it's generated  
✅ **Verifiable** - you can see exactly where answers come from  
✅ **Honest** - says "I don't know" if documents don't have answer  

That's it. Simple flow, clear stages, working system.
