import { useState, useCallback, useMemo, useEffect } from 'react'
import {
  ReactFlow,
  Node,
  Edge,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  Handle,
  Position,
  BackgroundVariant,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

interface Props {
  tree: any
  onChange?: (tree: any) => void
}

// --- Custom Node Components ---

function ConditionNode({ data }: { data: any }) {
  return (
    <div className="flow-node flow-condition">
      <Handle type="target" position={Position.Top} />
      <div className="flow-node-header">Condition</div>
      <div className="flow-node-body">
        <input
          className="flow-input"
          value={data.field || ''}
          onChange={e => data.onUpdate({ ...data.cond, field: e.target.value })}
          placeholder="field (e.g. user.age)"
        />
        <select
          className="flow-select"
          value={data.op || 'eq'}
          onChange={e => data.onUpdate({ ...data.cond, op: e.target.value })}
        >
          <option value="eq">eq</option>
          <option value="neq">neq</option>
          <option value="gt">gt</option>
          <option value="gte">gte</option>
          <option value="lt">lt</option>
          <option value="lte">lte</option>
          <option value="in">in</option>
          <option value="nin">nin</option>
          <option value="regex">regex</option>
          <option value="pct">pct</option>
        </select>
        <input
          className="flow-input"
          value={data.valueStr || ''}
          onChange={e => {
            let parsed: any = e.target.value
            try { parsed = JSON.parse(e.target.value) } catch {}
            data.onUpdate({ ...data.cond, value: parsed })
          }}
          placeholder="value"
        />
      </div>
      <Handle type="source" position={Position.Bottom} id="then" style={{ left: '30%' }} />
      <Handle type="source" position={Position.Bottom} id="else" style={{ left: '70%' }} />
      <div className="flow-labels">
        <span className="flow-label-true">then</span>
        <span className="flow-label-false">else</span>
      </div>
    </div>
  )
}

function ValueNode({ data }: { data: any }) {
  return (
    <div className="flow-node flow-value">
      <Handle type="target" position={Position.Top} />
      <div className="flow-node-header">Value</div>
      <div className="flow-node-body">
        <input
          className="flow-input"
          value={data.valueStr || ''}
          onChange={e => {
            let parsed: any = e.target.value
            try { parsed = JSON.parse(e.target.value) } catch {}
            data.onValueChange(parsed, e.target.value)
          }}
          placeholder="value (e.g. true, &quot;text&quot;, 42)"
        />
      </div>
    </div>
  )
}

function CombinatorNode({ data }: { data: any }) {
  return (
    <div className="flow-node flow-combinator">
      <Handle type="target" position={Position.Top} />
      <div className="flow-node-header">{(data.combinator || 'AND').toUpperCase()}</div>
      <div className="flow-node-body">
        <select
          className="flow-select"
          value={data.combinator || 'and'}
          onChange={e => data.onUpdate(e.target.value)}
        >
          <option value="and">AND</option>
          <option value="or">OR</option>
        </select>
        <span className="flow-count">{data.count} condition(s)</span>
      </div>
      <Handle type="source" position={Position.Bottom} id="then" style={{ left: '30%' }} />
      <Handle type="source" position={Position.Bottom} id="else" style={{ left: '70%' }} />
      <div className="flow-labels">
        <span className="flow-label-true">then</span>
        <span className="flow-label-false">else</span>
      </div>
    </div>
  )
}

const nodeTypes = {
  conditionNode: ConditionNode,
  valueNode: ValueNode,
  combinatorNode: CombinatorNode,
}

// --- Tree <-> Flow conversion ---

let nodeIdCounter = 0

function treeToFlow(
  node: any,
  parentId: string | null,
  handleId: string | null,
  x: number,
  y: number,
  onTreeChange: () => void,
  treeRef: { current: any }
): { nodes: Node[]; edges: Edge[] } {
  if (!node) return { nodes: [], edges: [] }

  const id = `node_${nodeIdCounter++}`
  const nodes: Node[] = []
  const edges: Edge[] = []

  if (parentId && handleId) {
    edges.push({
      id: `${parentId}-${handleId}-${id}`,
      source: parentId,
      target: id,
      sourceHandle: handleId,
      label: handleId,
      style: { stroke: handleId === 'then' ? '#3fb950' : '#f85149' },
    })
  }

  // Leaf node
  if (node.value !== undefined && !node.condition) {
    const valueStr = JSON.stringify(node.value)
    nodes.push({
      id,
      type: 'valueNode',
      position: { x, y },
      data: {
        valueStr,
        onValueChange: (parsed: any, raw: string) => {
          node.value = parsed
          treeRef.current = { ...treeRef.current }
          onTreeChange()
        },
      },
    })
    return { nodes, edges }
  }

  // Branch node
  if (node.condition) {
    const cond = node.condition

    if (cond.combinator) {
      // Combinator node
      nodes.push({
        id,
        type: 'combinatorNode',
        position: { x, y },
        data: {
          combinator: cond.combinator,
          count: cond.conditions?.length || 0,
          onUpdate: (newCombinator: string) => {
            cond.combinator = newCombinator
            treeRef.current = { ...treeRef.current }
            onTreeChange()
          },
        },
      })
    } else {
      // Simple condition node
      nodes.push({
        id,
        type: 'conditionNode',
        position: { x, y },
        data: {
          field: cond.field,
          op: cond.op,
          valueStr: JSON.stringify(cond.value),
          cond,
          onUpdate: (updated: any) => {
            Object.assign(cond, updated)
            treeRef.current = { ...treeRef.current }
            onTreeChange()
          },
        },
      })
    }

    // Then branch
    const thenResult = treeToFlow(node.then, id, 'then', x - 150, y + 150, onTreeChange, treeRef)
    nodes.push(...thenResult.nodes)
    edges.push(...thenResult.edges)

    // Else branch
    const elseResult = treeToFlow(node.else, id, 'else', x + 150, y + 150, onTreeChange, treeRef)
    nodes.push(...elseResult.nodes)
    edges.push(...elseResult.edges)
  }

  return { nodes, edges }
}

export default function TreeEditor({ tree, onChange }: Props) {
  const treeRef = useMemo(() => ({ current: null as any }), [])

  const parsedTree = useMemo(() => {
    try {
      const parsed = typeof tree === 'string' ? JSON.parse(tree) : tree
      treeRef.current = JSON.parse(JSON.stringify(parsed)) // deep clone
      return parsed
    } catch {
      return null
    }
  }, [tree])

  const triggerChange = useCallback(() => {
    if (onChange && treeRef.current) {
      onChange(JSON.parse(JSON.stringify(treeRef.current)))
    }
  }, [onChange])

  const { initialNodes, initialEdges } = useMemo(() => {
    if (!parsedTree) return { initialNodes: [], initialEdges: [] }
    nodeIdCounter = 0
    const { nodes, edges } = treeToFlow(
      JSON.parse(JSON.stringify(parsedTree)),
      null, null, 300, 50,
      triggerChange, treeRef
    )
    return { initialNodes: nodes, initialEdges: edges }
  }, [parsedTree])

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)

  // Sync when tree changes externally
  useEffect(() => {
    setNodes(initialNodes)
    setEdges(initialEdges)
  }, [initialNodes, initialEdges])

  if (!parsedTree) {
    return <div className="empty-state">Invalid tree JSON</div>
  }

  return (
    <div className="tree-editor-container">
      <div className="tree-editor-hint">
        Drag nodes to rearrange. Edit values directly in nodes. Changes sync to JSON editor on save.
      </div>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        minZoom={0.3}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
      >
        <Controls />
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="#333" />
      </ReactFlow>
    </div>
  )
}
