from langchain_community.vectorstores import FAISS
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_core.documents import Document

class Vectorstore:
    def __init__(self):
        self._embedding_model = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")
        self._vector_store = None

    def create_index(self, documents = []):
        if len(documents) == 0:
            return

        vectors = [
            Document(
                page_content=x["logs"],
                metadata={
                    "name": x["name"],
                    "plan_id": x["plan_id"],
                    "timestamp": x["timestamp"],
                    "label": x["label"],
                },
            )
            for x in documents
        ]

        self._vector_store = FAISS.from_documents(vectors, self._embedding_model)
    
    def query(self, sentence):
        if self._vector_store == None or sentence == None or sentence == "None":
            return []
    
        responses = self._vector_store.similarity_search_with_relevance_scores(
            sentence, k=4
        )

        labels = list()
        visited = set()

        for response in responses:
            data, score = response
            if data.metadata["label"] in visited or score < 0:
                continue

            labels.append({"label": data.metadata["label"], "score": score})
            visited.add(data.metadata["label"])

        return labels
