import React, { useState } from "react";

export function Counter() {
  const [value, setValue] = useState(0);

  return (
    <div>
      <div>Counter: {value}</div>
      <button onClick={() => setValue(value + 1)}>Increment</button>
      <button onClick={() => setValue(value - 1)}>Decrement</button>
    </div>
  );
}
