from typing import Optional, List
from pydantic import BaseModel

class TableColumn(BaseModel):
    name: str
    type: str
    is_pk: bool = False
    is_fk: bool = False
    constraint: Optional[str] = None

class NodeData(BaseModel):
    label: str
    subText: Optional[str] = None
    columns: Optional[List[TableColumn]] = None
    lane: Optional[str] = None
    icon: Optional[str] = None

class GraphNode(BaseModel):
    id: str
    type: str
    data: NodeData

class GraphEdge(BaseModel):
    id: str
    source: str
    target: str
    label: Optional[str] = None
    dashed: bool = False

class GraphPayload(BaseModel):
    nodes: List[GraphNode]
    edges: List[GraphEdge]
