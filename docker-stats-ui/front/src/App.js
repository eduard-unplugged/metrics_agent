import React, { useEffect, useState } from "react";

function App() {
  const [statsList, setStatsList] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // В продакшене можно использовать ENV-переменные или proxy
  const BACKEND_URL = "http://localhost:3000";

  const fetchStats = async () => {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetch(`${BACKEND_URL}/api/docker-stats`);
      if (!resp.ok) {
        throw new Error(`Failed to fetch stats, status=${resp.status}`);
      }
      const data = await resp.json();
      setStatsList(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handlePrune = async (instanceID) => {
    try {
      const resp = await fetch(`${BACKEND_URL}/api/prune?instance=${instanceID}`, {
        method: "POST",
      });
      if (!resp.ok) {
        throw new Error(`Prune failed, status=${resp.status}`);
      }
      // Успешный prune -> обновим список
      await fetchStats();
      alert("Prune done");
    } catch (err) {
      alert(err.message);
    }
  };

  useEffect(() => {
    fetchStats();
  }, []);

  return (
    <div style={{ padding: "1rem" }}>
      <h1>Docker Stats Dashboard</h1>
      {loading && <p>Loading...</p>}
      {error && <p style={{ color: "red" }}>Ошибка: {error}</p>}

      <button onClick={fetchStats} disabled={loading}>
        Refresh
      </button>

      <table
        border="1"
        cellPadding="8"
        cellSpacing="0"
        style={{ marginTop: "1rem", borderCollapse: "collapse" }}
      >
        <thead>
          <tr>
            <th>Instance ID</th>
            <th>Images Size (GB)</th>
            <th>Timestamp</th>
            <th>Prune Action</th>
            <th>Manual Prune</th>
          </tr>
        </thead>
        <tbody>
          {statsList.map((st) => (
            <tr key={st.instance_id}>
              <td>{st.instance_id}</td>
              <td>{st.images_size_gb.toFixed(2)}</td>
              <td>{st.timestamp}</td>
              <td>{st.prune_action ? "Yes" : "No"}</td>
              <td>
                <button onClick={() => handlePrune(st.instance_id)}>Prune</button>
              </td>
            </tr>
          ))}
          {statsList.length === 0 && (
            <tr>
              <td colSpan="5">No data yet</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

export default App;