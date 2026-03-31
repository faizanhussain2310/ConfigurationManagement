import { useState, useEffect, useCallback } from 'react'
import { api, Rule } from '../api/client'

export function useRules() {
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchRules = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.listRules()
      setRules(data.rules)
      setError(null)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchRules()
  }, [fetchRules])

  return { rules, loading, error, refresh: fetchRules }
}
