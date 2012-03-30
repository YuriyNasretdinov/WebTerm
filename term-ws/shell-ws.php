<?php
$PORT = 13923; // port terminal daemon will be run at
$PASSWORD = "some password for me, for example this file location: ".__FILE__;

error_reporting(E_ALL);
header('Content-type: text/html; charset: UTF-8');
ini_set('display_errors', 1);
$rcfile = escapeshellarg(dirname(__FILE__).'/bashrc');
chdir(dirname(__FILE__));
?>
<!DOCTYPE html>
<html>
<head>
	<title>Terminal</title>
</head>
<body onresize="window.resize &amp;&amp; window.scr &amp;&amp; resize(scr, ws)">
<style>
    body, table, .screen { margin: 0px; padding: 0px; background: black; }
    .outputrow {
        font-family: fixed, "courier new", courier, monospace;
        margin: 0px;
        line-height: 16px;
        font-size: 15px;
        color: white;
    }
    span { margin:0px; padding: 0px; border: 0px; }
</style>
<script src="pyte/js/charsets.js"></script>
<script src="pyte/js/control.js"></script>
<script src="pyte/js/escape.js"></script>
<script src="pyte/js/graphics.js"></script>
<script src="pyte/js/modes.js"></script>
<script src="pyte/js/screens.js"></script>
<script src="pyte/js/streams.js"></script>
<script src="term-ws.js"></script>
<audio src="bell.ogg" id="bell" style="display: none;"></audio>
<div id="screen">
</div>
<div class="copy-paste"><table cellpadding="0" cellspacing="0" width="100%">
    <tr>
        <td><input id="paste-buf" style="width: 100%;" onkeydown="if (event.keyCode == 13) paste();"
                   placeholder="You can paste stuff you need here" type="password" /></td>
        <td width="50"><input type="button" value="paste" onclick="paste()" /></td>
    </tr>
</table></div>
<?php
putenv('PATH='.getenv('PATH').':/usr/local/bin');
system('exec nohup ./ws bashrc '.$PORT.' '.md5($PASSWORD).' '.strlen($PASSWORD).' </dev/null >>ws.log 2>&1 &');
?>
<script>
	var colsrows = getWindowColsRows()
	var stream = new Stream();
	var scr = new Screen(colsrows[0], colsrows[1]);
	stream.attach(scr);
	
	var ws = new WebSocket('ws://' + window.location.host + ':<?=$PORT?>/ws', "term")
	
	ws.onopen = function() {
		ws.send(<?=json_encode($PASSWORD)?>)
		ws.send(indent(colsrows[0], 8))
		ws.send(indent(colsrows[1], 8))
	}
	ws.onmessage = function(ev) {
		stream.feed(ev.data)
		newData = true
	}
	ws.onclose = function() {
		stream.feed("Connection closed\n")
		newData = true
		clearInterval(blinkInterv);
	}
	
	var term = new Term(send_cmd);
	term.open()
	
	function send_cmd(val) {
		val = '' + val
	    ws.send('i' + indent(val.length, 8) + val)
	}
	
    function redraw() {
        for (var i = 0; i < scr.lines; i++) {
            redraw_line(scr, i);
        }
    }

	var newData = false
	
	setInterval(function() {
		if (newData) {
			redraw()
			newData = false
		}
	}, 16)
	
	resize(scr, ws, true)
</script>
</body></html>