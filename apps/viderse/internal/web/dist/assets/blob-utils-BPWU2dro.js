async function f(r){return typeof r.arrayBuffer=="function"?r.arrayBuffer():new Response(r).arrayBuffer()}export{f as b};
