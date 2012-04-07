"use strict";
/* add the bind function if not present */
if (!Function.prototype.bind) {
	Function.prototype.bind = function(obj) {
		var slice1 = [].slice,
		args = slice1.call(arguments, 1),
		self = this,
		nop = function () {},
		bound = function () {
			return self.apply( this instanceof nop ? this : ( obj || {} ),
							   args.concat( slice1.call(arguments) ) );
		};

		nop.prototype = self.prototype;

		bound.prototype = new nop();

		return bound;
	};
}


function Term(ca) {
	this.handler = ca;
	this.is_mac = (navigator.userAgent.indexOf("Mac") >= 0) ? true : false;
	this.key_rep_state = 0;
	this.key_rep_str = "";
}
Term.prototype.open = function () {
	document.addEventListener("keydown", this.keyDownHandler.bind(this), true);
	document.addEventListener("keypress", this.keyPressHandler.bind(this), true);
};
Term.prototype.keyDownHandler = function (ev) {
	var tagName = ev.target && ev.target.tagName;
	if (tagName == 'INPUT' || tagName == 'TEXTAREA') return;

	var seq;
	seq = "";
	switch (ev.keyCode) {
		case 8: // backspace
			seq = "\x08";
			break;
		case 9: // tab
			seq = "\t";
			break;
		case 13: // enter
			seq = "\r";
			break;
		case 27: // esc
			seq = "\x1b";
			break;
		case 37: // arrow left
			if (ev.altKey) seq = "\x1bb";
			else		   seq = "\x1b[D";
			break;
		case 39: // arrow right
			if (ev.altKey) seq = "\x1bf";
			else		   seq = "\x1b[C";
			break;
		case 38: // arrow up
			seq = "\x1b[A";
			break;
		case 40: // arrow down
			seq = "\x1b[B";
			break;
		case 46: // delete
			seq = "\x1b[3~";
			break;
		case 45: // insert
			seq = "\x1b[2~";
			break;
		case 36: // home
			seq = "\x1bOH";
			break;
		case 35: // end
			seq = "\x1bOF";
			break;
		case 33: // page up
			seq = "\x1b[5~";
			break;
		case 34: // page down
			seq = "\x1b[6~";
			break;
		case 112: // F1
			seq = "\x1b[[A";
			break;
		case 113: // F2
			seq = "\x1b[[B";
			break;
		case 114: // F3
			seq = "\x1b[[C";
			break;
		case 115: // F4
			seq = "\x1b[[D";
			break;
		case 116: // F5
			seq = "\x1b[15~";
			break;
		case 117: // F6
			seq = "\x1b[17~";
			break;
		case 118: // F7
			seq = "\x1b[18~";
			break;
		case 119: // F8
			seq = "\x1b[19~";
			break;
		case 120: // F9
			seq = "\x1b[20~";
			break;
		case 121: // F10
			seq = "\x1b[21~";
			break;
		case 122: // F11
			seq = "\x1b[23~";
			break;
		case 123: // F12
			seq = "\x1b[24~";
			break;
		default:
			if (ev.ctrlKey) {
				if (ev.keyCode >= 65 && ev.keyCode <= 90) {
					seq = String.fromCharCode(ev.keyCode - 64);
				} else if (ev.keyCode == 32) {
					seq = String.fromCharCode(0);
				}
			} else if ((!this.is_mac && ev.altKey) || (this.is_mac && ev.metaKey)) {
				if (ev.keyCode >= 65 && ev.keyCode <= 90) {
					seq = "\x1b" + String.fromCharCode(ev.keyCode + 32);
				}
			}
			break;
	}
	if (seq) {
		if (ev.stopPropagation) ev.stopPropagation();
		if (ev.preventDefault) ev.preventDefault();
		this.key_rep_state = 1;
		this.key_rep_str = seq;
		this.handler(seq);
		return false;
	} else {
		this.key_rep_state = 0;
		return true;
	}
};
Term.prototype.keyPressHandler = function (ev) {
	var tagName = ev.target && ev.target.tagName;
	if (tagName == 'INPUT' || tagName == 'TEXTAREA') return;

	var seq, code;
	if (ev.stopPropagation) ev.stopPropagation();
	if (ev.preventDefault) ev.preventDefault();
	seq = "";
	if (!("charCode" in ev)) {
		code = ev.keyCode;
		if (this.key_rep_state == 1) {
			this.key_rep_state = 2;
			return false;
		} else if (this.key_rep_state == 2) {
			this.handler(this.key_rep_str);
			return false;
		}
	} else {
		code = ev.charCode;
	}
	if (code != 0) {
		if (!ev.ctrlKey && ((!this.is_mac && !ev.altKey) || (this.is_mac && !ev.metaKey))) {
			seq = String.fromCharCode(code);
		}
	}
	if (seq) {
		this.handler(seq);
		return false;
	} else {
		return true;
	}
};

function indent(str, len) {
	str = '' + str
	while (str.length < 8) str += ' '
	return str
}

function paste() {
	var el = document.getElementById('paste-buf');
	send_cmd(el.value+"\n");
	el.value = '';
	try { document.body.focus(); } catch(e) { }
	try { document.focus(); } catch(e) { }
	try { window.focus(); } catch(e) { }
}

var cursor_is_visible = true;
var cursor = {x: 0, y: 0};
var blinkInterv = setInterval(function() {
	var el = document.getElementById('cursor');
	if (!el) return;
	cursor_is_visible = !cursor_is_visible;
	el.style.backgroundColor = cursor_is_visible ? 'grey' : '';
}, 1000);

var html_esc = {
	'<': '&lt;',
	'>': '&gt;',
	'&': '&amp;',
	' ': '&nbsp;'
};

var drawn_lines = [];
var color_map = {
	'black': 'black',
	'red': '#ff0000',
	'green': '#00ff00',
	'brown': '#ffc709',
	'blue': '#006fb8',
	'magenta': '#ff00ff',
	'cyan': '#2cb5e9',
	'white': '#ffffff'
}

function redraw_line(screen, line) {
	cursor = screen.cursor;
	var el = document.getElementById('row'+line);
	var chars = screen[line], ch;
	var res = ['<span>'];
	var prev_style = '', style, i;
	for (i = 0; i < chars.length; i++) {
		ch = chars[i];

		style = '';
		if (ch.fg != 'default') style += 'color: '+(color_map[ch.fg] || ch.fg)+'; ';
		if (ch.bg != 'default') style += 'background-color: '+(color_map[ch.bg] || ch.bg)+'; ';
		if (ch.fg == 'black' && (ch.bg == 'default' || ch.bg == 'black')) style += 'color: gray; '
		if (ch.bold) style += 'font-weight: bold; ';
		if (ch.italics) style += 'font-style: italic; ';
		if (ch.underscore || ch.strikethrough) {
			style += 'font-decoration: '+(ch.underscore ? 'underline ' : '')+' '+(ch.underscore ? 'line-through ' : '')+'; ';
		}

		if (style != prev_style) {
			res.push('</span><span style="' + style + '">');
		}

		if (cursor.x == i && cursor.y == line) res.push('<span id="cursor" style="background-color: grey;">');

		res.push(html_esc[ch.data] || ch.data);

		if (cursor.x == i && cursor.y == line) res.push('</span>');

		prev_style = style;
	}
	res.push('</span>');

	var line_html = res.join('');

	if (!drawn_lines[line] || drawn_lines[line] != line_html) el.innerHTML = line_html;
	drawn_lines[line] = line_html;
}

function window_cols_rows() {
	var winW = 630, winH = 460;
	if (document.body && document.body.offsetWidth) {
		winW = document.body.offsetWidth;
		winH = document.body.offsetHeight;
	}
	if (document.compatMode=='CSS1Compat' && document.documentElement && document.documentElement.offsetWidth) {
		winW = document.documentElement.offsetWidth;
		winH = document.documentElement.offsetHeight;
	}
	if (window.innerWidth && window.innerHeight) {
		winW = window.innerWidth;
		winH = window.innerHeight;
	}

	return [Math.floor(winW / 9), Math.floor( (winH - 30) / 16)]
}

function resize(scr, ws, initonly) {
	var colsrows = window_cols_rows()
	var rows = []
	for(var i = 0; i < colsrows[1]; i++) {
		rows.push("<div id='row" + i + "' class='outputrow'>&nbsp;</div>")
	}
	document.getElementById('screen').innerHTML = rows.join("\n")
	if (!initonly) {
		scr.resize(colsrows[1], colsrows[0])
		drawn_lines = []
		newData = true

		ws.send('w' + indent(colsrows[0], 8) + indent(colsrows[1], 8))	
	}
}

function string_utf8_len(str) {
	var len = 0, l = str.length;

	for (var i = 0; i < l; i++) {
		var c = str.charCodeAt(i);
		if (c <= 0x0000007F) len++;
		else if (c >= 0x00000080 && c <= 0x000007FF) len += 2;
		else if (c >= 0x00000800 && c <= 0x0000FFFF) len += 3;
		else len += 4;
	}

	return len;
}