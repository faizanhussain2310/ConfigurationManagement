import { useMemo } from 'react'
import Tree from 'react-d3-tree'

interface Props {
  tree: any
}

interface TreeNode {
  name: string
  attributes?: Record<string, string>
  children?: TreeNode[]
}

function buildTreeData(node: any): TreeNode {
  if (!node) return { name: '(empty)' }

  // Leaf node
  if (node.value !== undefined) {
    return {
      name: `${JSON.stringify(node.value)}`,
      attributes: { type: 'leaf' },
    }
  }

  // Branch node with condition
  if (node.condition) {
    const condLabel = formatCondition(node.condition)
    const children: TreeNode[] = []

    if (node.then) {
      children.push({
        name: 'Then',
        attributes: { branch: 'true' },
        children: [buildTreeData(node.then)],
      })
    }
    if (node.else) {
      children.push({
        name: 'Else',
        attributes: { branch: 'false' },
        children: [buildTreeData(node.else)],
      })
    }

    return {
      name: condLabel,
      attributes: { type: 'condition' },
      children,
    }
  }

  return { name: '(unknown)' }
}

function formatCondition(cond: any): string {
  if (cond.combinator) {
    return `${cond.combinator.toUpperCase()} (${cond.conditions?.length || 0})`
  }
  const val = Array.isArray(cond.value) ? `[${cond.value.join(', ')}]` : JSON.stringify(cond.value)
  return `${cond.field} ${cond.op} ${val}`
}

export default function TreeView({ tree }: Props) {
  const data = useMemo(() => {
    try {
      const parsed = typeof tree === 'string' ? JSON.parse(tree) : tree
      return buildTreeData(parsed)
    } catch {
      return { name: '(invalid tree)' }
    }
  }, [tree])

  return (
    <div className="tree-container">
      <Tree
        data={data}
        orientation="vertical"
        pathFunc="step"
        translate={{ x: 300, y: 50 }}
        separation={{ siblings: 1.5, nonSiblings: 2 }}
        nodeSize={{ x: 200, y: 100 }}
        collapsible={false}
      />
    </div>
  )
}
